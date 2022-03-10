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

package vault

import (
	"context"
	"crypto/x509"
	"net/http"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/vault/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	kvclient "github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/client"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errNoConfig        = "no Vault config provided"
	errNewClient       = "cannot create new client"
	errExtractCABundle = "cannot extract ca bundle"
	errAppendCABundle  = "cannot append ca bundle"
	errExtractToken    = "cannot extract token"
	errNoTokenProvided = "token auth configured but no token provided"

	errGet    = "cannot get secret"
	errApply  = "cannot apply secret"
	errDelete = "cannot delete secret"
)

// KVClient is a Vault AdditiveKVClient Secrets engine client that supports both v1 and v2.
type KVClient interface {
	Get(path string, secret *kvclient.KVSecret) error
	Apply(path string, secret *kvclient.KVSecret, ao ...kvclient.ApplyOption) error
	Delete(path string) error
}

// SecretStore is a Vault Secret Store.
type SecretStore struct {
	client KVClient

	defaultParentPath string
}

// NewSecretStore returns a new Vault SecretStore.
func NewSecretStore(ctx context.Context, kube client.Client, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	if cfg.Vault == nil {
		return nil, errors.New(errNoConfig)
	}
	vCfg := api.DefaultConfig()
	vCfg.Address = cfg.Vault.Server

	if cfg.Vault.CABundle != nil {
		ca, err := resource.CommonCredentialExtractor(ctx, cfg.Vault.CABundle.Source, kube, cfg.Vault.CABundle.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errExtractCABundle)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(ca); !ok {
			return nil, errors.Wrap(err, errAppendCABundle)
		}
		vCfg.HttpClient.Transport.(*http.Transport).TLSClientConfig.RootCAs = pool
	}

	c, err := api.NewClient(vCfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	switch cfg.Vault.Auth.Method {
	case v1.VaultAuthToken:
		if cfg.Vault.Auth.Token == nil {
			return nil, errors.New(errNoTokenProvided)
		}
		t, err := resource.CommonCredentialExtractor(ctx, cfg.Vault.Auth.Token.Source, kube, cfg.Vault.Auth.Token.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errExtractToken)
		}
		c.SetToken(string(t))
	default:
		return nil, errors.Errorf("%q is not supported as an auth method", cfg.Vault.Auth.Method)
	}

	return &SecretStore{
		client:            kvclient.NewAdditiveClient(c.Logical(), cfg.Vault.MountPath, kvclient.WithVersion(cfg.Vault.Version)),
		defaultParentPath: cfg.DefaultScope,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Vault Secret.
func (ss *SecretStore) ReadKeyValues(_ context.Context, n store.ScopedName, s *store.Secret) error {
	kvs := &kvclient.KVSecret{}
	if err := ss.client.Get(ss.path(n), kvs); resource.Ignore(kvclient.IsNotFound, err) != nil {
		return errors.Wrap(err, errGet)
	}

	kv := make(store.KeyValues, len(kvs.Data))
	for k, v := range kvs.Data {
		kv[k] = []byte(v.(string))
	}
	s.ScopedName = n
	s.Data = kv
	if len(kvs.CustomMeta) > 0 {
		s.Metadata = &v1.ConnectionSecretMetadata{
			Labels: make(map[string]string, len(kvs.CustomMeta)),
		}
	}
	for k, v := range kvs.CustomMeta {
		if val, ok := v.(string); ok {
			s.Metadata.Labels[k] = val
		}
	}
	return nil
}

// WriteKeyValues writes key value pairs to a given Vault Secret.
func (ss *SecretStore) WriteKeyValues(_ context.Context, s *store.Secret, wo ...store.WriteOption) (changed bool, err error) {
	data := make(map[string]interface{}, len(s.Data))
	for k, v := range s.Data {
		data[k] = string(v)
	}

	kvSecret := &kvclient.KVSecret{}
	kvSecret.Data = data
	if s.Metadata != nil && len(s.Metadata.Labels) > 0 {
		kvSecret.CustomMeta = make(map[string]interface{}, len(s.Metadata.Labels))
		for k, v := range s.Metadata.Labels {
			kvSecret.CustomMeta[k] = v
		}
	}

	ao := applyOptions(wo...)
	ao = append(ao, kvclient.AllowUpdateIf(func(current, desired *kvclient.KVSecret) bool {
		return !cmp.Equal(current, desired, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(kvclient.KVSecret{}))
	}))

	err = ss.client.Apply(ss.path(s.ScopedName), kvSecret, ao...)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errApply)
	}
	return true, nil
}

// DeleteKeyValues delete key value pairs from a given Vault Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no Data left.
func (ss *SecretStore) DeleteKeyValues(_ context.Context, s *store.Secret) error {
	kvSecret := &kvclient.KVSecret{}
	err := ss.client.Get(ss.path(s.ScopedName), kvSecret)
	if kvclient.IsNotFound(err) {
		// Secret already deleted, nothing to do.
		return nil
	}
	if err != nil {
		return errors.Wrap(err, errGet)
	}
	for k := range s.Data {
		delete(kvSecret.Data, k)
	}
	if len(s.Data) == 0 || len(kvSecret.Data) == 0 {
		// Secret is deleted only if:
		// - No kv to delete specified as input
		// - No data left in the secret
		return errors.Wrap(ss.client.Delete(ss.path(s.ScopedName)), errDelete)
	}
	// If there are still keys left, update the secret with the remaining.
	return errors.Wrap(ss.client.Apply(ss.path(s.ScopedName), kvSecret), errApply)
}

func (ss *SecretStore) path(s store.ScopedName) string {
	if s.Scope != "" {
		return filepath.Join(s.Scope, s.Name)
	}
	return filepath.Join(ss.defaultParentPath, s.Name)
}

func applyOptions(wo ...store.WriteOption) []kvclient.ApplyOption {
	ao := make([]kvclient.ApplyOption, len(wo))
	for i := range wo {
		o := wo[i]
		ao[i] = func(current, desired *kvclient.KVSecret) error {
			return o(context.Background(), &store.Secret{
				Metadata: &v1.ConnectionSecretMetadata{
					Labels: labelsFromCustomMetadata(current.CustomMeta),
				},
				Data: keyValuesFromData(current.Data),
			}, &store.Secret{
				Metadata: &v1.ConnectionSecretMetadata{
					Labels: labelsFromCustomMetadata(desired.CustomMeta),
				},
				Data: keyValuesFromData(desired.Data),
			})
		}
	}
	return ao
}

func labelsFromCustomMetadata(meta map[string]interface{}) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	l := make(map[string]string, len(meta))
	for k := range meta {
		if val, ok := meta[k].(string); ok {
			l[k] = val
		}
	}
	return l
}

func keyValuesFromData(data map[string]interface{}) store.KeyValues {
	if len(data) == 0 {
		return nil
	}
	kv := make(store.KeyValues, len(data))
	for k := range data {
		if val, ok := data[k].(string); ok {
			kv[k] = []byte(val)
		}
	}
	return kv
}
