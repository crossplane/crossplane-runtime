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
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errNoConfig     = "no Vault config provided"
	errNewClient    = "cannot create new client"
	errExtractToken = "cannot extract token"

	errRead         = "cannot read secret"
	errReadToAppend = "cannot read secret to append keys"
	errWrite        = "cannot write secret"
	errDelete       = "cannot delete secret"
)

// SecretStore is a Vault Secret Store.
type SecretStore struct {
	client Client

	pathPrefix        string
	defaultParentPath string
}

// NewSecretStore returns a new Vault SecretStore.
func NewSecretStore(ctx context.Context, kube client.Client, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	if cfg.Vault == nil {
		return nil, errors.New(errNoConfig)
	}
	if cfg.Vault.Auth.Method != v1.VaultAuthToken {
		return nil, errors.Errorf("%q auth not supported yet, please use Token auth", cfg.Vault.Auth.Method)
	}
	if cfg.Vault.Auth.Token == nil {
		return nil, errors.New("token auth configured but no token provided")
	}
	vCfg := api.DefaultConfig()
	vCfg.Address = cfg.Vault.Server

	c, err := api.NewClient(vCfg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	t, err := resource.CommonCredentialExtractor(ctx, cfg.Vault.Auth.Token.Source, kube, cfg.Vault.Auth.Token.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errExtractToken)
	}

	c.SetToken(string(t))

	return &SecretStore{
		client: c.Logical(),

		pathPrefix:        cfg.Vault.MountPath,
		defaultParentPath: cfg.DefaultScope,
	}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Vault Secret.
func (ss *SecretStore) ReadKeyValues(_ context.Context, i store.Secret) (store.KeyValues, error) {
	// TODO(turkenh): Handle not found
	s, err := ss.client.Read(ss.pathForSecretInstance(i))
	if err != nil {
		return nil, errors.Wrap(err, errRead)
	}
	// TODO(turkenh): debug log s.Warnings ?
	kv := make(map[string][]byte, len(s.Data))
	for k, v := range s.Data {
		kv[k] = []byte(v.(string))
	}
	return kv, nil
}

// WriteKeyValues writes key value pairs to a given Vault Secret.
func (ss *SecretStore) WriteKeyValues(_ context.Context, i store.Secret, kv store.KeyValues) error {
	if len(kv) == 0 {
		// Nothing to write
		return nil
	}
	s, err := ss.client.Read(ss.pathForSecretInstance(i))
	if err != nil {
		return errors.Wrap(err, errReadToAppend)
	}

	var existing map[string]interface{}
	if s != nil {
		existing = s.Data
	}
	data := make(map[string]interface{}, len(kv)+len(existing))
	for k, v := range kv {
		// Note(turkenh): value here is or type []byte, it is stored as base64
		// encoded in Vault if we don't cast it to string. This could be a
		// configuration option if needed.
		data[k] = string(v)
	}
	for k, v := range existing {
		data[k] = v
	}

	_, err = ss.client.Write(ss.pathForSecretInstance(i), data)
	// TODO(turkenh): debug log s.Warnings ?
	return errors.Wrap(err, errWrite)
}

// DeleteKeyValues delete key value pairs from a given Vault Secret.
// If no kv specified, the whole secret instance is deleted.
// If kv specified, those would be deleted and secret instance will be deleted
// only if there is no data left.
func (ss *SecretStore) DeleteKeyValues(_ context.Context, i store.Secret, kv store.KeyValues) error {
	_, err := ss.client.Delete(ss.pathForSecretInstance(i))
	return errors.Wrap(err, errDelete)
}

func (ss *SecretStore) pathForSecretInstance(i store.Secret) string {
	if i.Scope != "" {
		return filepath.Clean(filepath.Join(ss.pathPrefix, i.Scope, i.Name))
	}
	return filepath.Clean(filepath.Join(ss.pathPrefix, ss.defaultParentPath, i.Name))
}
