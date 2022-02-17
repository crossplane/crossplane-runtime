/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errGetSecret            = "cannot get secret"
	errDeleteSecret         = "cannot delete secret"
	errUpdateSecret         = "cannot update secret"
	errCreateOrUpdateSecret = "cannot create or update connection applicator"
	errParseMetadata        = "cannot parse metadata"

	errExtractKubernetesAuthCreds = "cannot extract kubernetes auth credentials"
)

type secretMetadata struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Type        corev1.SecretType `json:"type"`
}

// SecretStore is a Kubernetes Secret Store.
type SecretStore struct {
	client     client.Client
	applicator resource.Applicator

	defaultNamespace string
}

// NewSecretStore returns a new Kubernetes SecretStore.
func NewSecretStore(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	if cfg.Kubernetes == nil {
		// No KubernetesSecretStoreConfig provided, local API Server will be
		// used as Secret Store.
		return &SecretStore{
			client:           local,
			applicator:       resource.NewApplicatorWithRetry(resource.NewAPIPatchingApplicator(local), resource.IsAPIErrorWrapped, nil),
			defaultNamespace: cfg.DefaultScope,
		}, nil
	}

	// Configure client for an external API server with a given Kubeconfig.
	kfg, err := resource.CommonCredentialExtractor(ctx, cfg.Kubernetes.Auth.Source, local, cfg.Kubernetes.Auth.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractKubernetesAuthCreds)
	}
	remote, err := clientForKubeconfig(kfg)
	if err != nil {
		return nil, errors.Wrap(err, errExtractKubernetesAuthCreds)
	}

	return &SecretStore{
		client:           remote,
		applicator:       resource.NewApplicatorWithRetry(resource.NewAPIPatchingApplicator(remote), resource.IsAPIErrorWrapped, nil),
		defaultNamespace: cfg.DefaultScope,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Kubernetes Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, i store.Secret) (store.KeyValues, error) {
	s := &corev1.Secret{}
	return s.Data, errors.Wrap(ss.client.Get(ctx, types.NamespacedName{Name: i.Name, Namespace: ss.namespaceForSecret(i)}, s), errGetSecret)
}

// WriteKeyValues writes key value pairs to a given Kubernetes Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	meta := secretMetadata{}
	if len(i.Metadata) > 0 {
		if err := json.Unmarshal(i.Metadata, &meta); err != nil {
			return errors.Wrap(err, errParseMetadata)
		}
	}

	t := resource.SecretTypeConnection
	if meta.Type != "" {
		t = meta.Type
	}

	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        i.Name,
			Namespace:   ss.namespaceForSecret(i),
			Labels:      meta.Labels,
			Annotations: meta.Annotations,
		},
		Type: t,
		Data: kv,
	}

	return errors.Wrap(ss.applicator.Apply(ctx, s), errCreateOrUpdateSecret)
}

// DeleteKeyValues delete key value pairs from a given Kubernetes Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no data left.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	// NOTE(turkenh): DeleteKeyValues method wouldn't need to do anything if we
	// have used owner references similar to existing implementation. However,
	// this wouldn't work if the K8s API is not the same as where SecretOwner
	// object lives, i.e. a remote cluster.
	// Considering there is not much additional value with deletion via garbage
	// collection in this specific case other than one less API call during
	// deletion, I opted for unifying both instead of adding conditional logic
	// like add owner references if not remote and not call delete etc.
	s := &corev1.Secret{}
	err := ss.client.Get(ctx, types.NamespacedName{Name: i.Name, Namespace: ss.namespaceForSecret(i)}, s)
	if kerrors.IsNotFound(err) {
		// Secret already deleted, nothing to do.
		return nil
	}
	if err != nil {
		return errors.Wrap(err, errGetSecret)
	}
	// Delete all supplied keys from secret data
	for k := range kv {
		delete(s.Data, k)
	}
	if len(kv) == 0 || len(s.Data) == 0 {
		// Secret is deleted only if:
		// - No kv to delete specified as input
		// - No data left in the secret
		return errors.Wrapf(ss.client.Delete(ctx, s), errDeleteSecret)
	}
	// If there are still keys left, update the secret with the remaining.
	return errors.Wrapf(ss.client.Update(ctx, s), errUpdateSecret)
}

func (ss *SecretStore) namespaceForSecret(i store.Secret) string {
	if i.Scope == "" {
		return ss.defaultNamespace
	}
	return i.Scope
}
