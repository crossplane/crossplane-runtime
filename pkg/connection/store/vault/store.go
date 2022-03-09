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
	Apply(path string, secret *kvclient.KVSecret) error
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
func (ss *SecretStore) ReadKeyValues(_ context.Context, i store.Secret) (store.KeyValues, error) {
	s := &kvclient.KVSecret{}
	if err := ss.client.Get(ss.pathForSecretInstance(i), s); resource.Ignore(kvclient.IsNotFound, err) != nil {
		return nil, errors.Wrap(err, errGet)
	}
	kv := make(store.KeyValues, len(s.Data))
	for k, v := range s.Data {
		kv[k] = []byte(v.(string))
	}
	return kv, nil
}

// WriteKeyValues writes key value pairs to a given Vault Secret.
func (ss *SecretStore) WriteKeyValues(_ context.Context, i store.Secret, conn store.KeyValues) error {
	data := make(map[string]interface{}, len(conn))
	for k, v := range conn {
		data[k] = string(v)
	}

	kvSecret := &kvclient.KVSecret{}
	kvSecret.Data = data
	if i.Metadata != nil && len(i.Metadata.Labels) > 0 {
		kvSecret.CustomMeta = make(map[string]interface{}, len(i.Metadata.Labels))
		for k, v := range i.Metadata.Labels {
			kvSecret.CustomMeta[k] = v
		}
	}

	return errors.Wrap(ss.client.Apply(ss.pathForSecretInstance(i), kvSecret), errApply)
}

// DeleteKeyValues delete key value pairs from a given Vault Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no Data left.
func (ss *SecretStore) DeleteKeyValues(_ context.Context, i store.Secret, conn store.KeyValues) error {
	s := &kvclient.KVSecret{}
	err := ss.client.Get(ss.pathForSecretInstance(i), s)
	if kvclient.IsNotFound(err) {
		// Secret already deleted, nothing to do.
		return nil
	}
	if err != nil {
		return errors.Wrap(err, errGet)
	}
	for k := range conn {
		delete(s.Data, k)
	}
	if len(conn) == 0 || len(s.Data) == 0 {
		// Secret is deleted only if:
		// - No kv to delete specified as input
		// - No data left in the secret
		return errors.Wrap(ss.client.Delete(ss.pathForSecretInstance(i)), errDelete)
	}
	// If there are still keys left, update the secret with the remaining.
	return errors.Wrap(ss.client.Apply(ss.pathForSecretInstance(i), s), errApply)
}

func (ss *SecretStore) pathForSecretInstance(i store.Secret) string {
	if i.Scope != "" {
		return filepath.Join(i.Scope, i.Name)
	}
	return filepath.Join(ss.defaultParentPath, i.Name)
}
