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
	"path/filepath"

	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errNoConfig     = "no Vault config provided"
	errNewClient    = "cannot create new client"
	errExtractToken = "cannot extract token"

	errGet    = "cannot get secret"
	errApply  = "cannot apply secret"
	errDelete = "cannot delete secret"
)

// KVClient is a Vault KV Secrets engine client that supports both v1 and v2.
type KVClient interface {
	Get(path string, secret *KVSecret) error
	Apply(path string, secret *KVSecret) error
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

	c, err := api.NewClient(vCfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	switch cfg.Vault.Auth.Method {
	case v1.VaultAuthToken:
		if cfg.Vault.Auth.Token == nil {
			return nil, errors.New("token auth configured but no token provided")
		}
		t, err := resource.CommonCredentialExtractor(ctx, cfg.Vault.Auth.Token.Source, kube, cfg.Vault.Auth.Token.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, errExtractToken)
		}
		c.SetToken(string(t))
	case v1.VaultAuthKubernetes:
		return nil, errors.Errorf("%q is not supported as an auth method yet", v1.VaultAuthKubernetes)
	default:
		return nil, errors.Errorf("%q is not supported as an auth method", cfg.Vault.Auth.Method)
	}

	return &SecretStore{
		client:            NewKV(c.Logical(), cfg.Vault.MountPath, WithVersion(cfg.Vault.Version)),
		defaultParentPath: cfg.DefaultScope,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Vault Secret.
func (ss *SecretStore) ReadKeyValues(_ context.Context, i store.Secret) (store.KeyValues, error) {
	s := &KVSecret{}
	if err := ss.client.Get(ss.pathForSecretInstance(i), s); resource.Ignore(isNotFound, err) != nil {
		return nil, errors.Wrap(err, errGet)
	}
	kv := make(store.KeyValues, len(s.data))
	for k, v := range s.data {
		kv[k] = []byte(v.(string))
	}
	return kv, nil
}

// WriteKeyValues writes key value pairs to a given Vault Secret.
func (ss *SecretStore) WriteKeyValues(_ context.Context, i store.Secret, kv store.KeyValues) error {
	data := make(map[string]interface{}, len(kv))
	for k, v := range kv {
		data[k] = string(v)
	}

	kvSecret := &KVSecret{data: data}
	if i.Metadata != nil && len(i.Metadata.Labels) > 0 {
		kvSecret.customMeta = make(map[string]interface{}, len(i.Metadata.Labels))
		for k, v := range i.Metadata.Labels {
			kvSecret.customMeta[k] = v
		}
	}

	return errors.Wrap(ss.client.Apply(ss.pathForSecretInstance(i), kvSecret), errApply)
}

// DeleteKeyValues delete key value pairs from a given Vault Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no data left.
func (ss *SecretStore) DeleteKeyValues(_ context.Context, i store.Secret, kv store.KeyValues) error {
	// TODO(turkenh): Handle deletion of partial kv, currently we delete
	//  whole secret.
	return errors.Wrap(ss.client.Delete(ss.pathForSecretInstance(i)), errDelete)
}

func (ss *SecretStore) pathForSecretInstance(i store.Secret) string {
	if i.Scope != "" {
		return filepath.Clean(filepath.Join(i.Scope, i.Name))
	}
	return filepath.Clean(filepath.Join(ss.defaultParentPath, i.Name))
}
