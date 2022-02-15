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

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
)

// SecretStore is a Vault Secret Store.
type SecretStore struct{}

// NewSecretStore returns a new Vault SecretStore.
func NewSecretStore(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (*SecretStore, error) {
	return &SecretStore{}, nil
}

// ReadKeyValues reads and returns key value pairs for a given Vault Secret.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, i store.Secret) (store.KeyValues, error) {
	panic("implement me")
}

// WriteKeyValues writes key value pairs to a given Vault Secret.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	panic("implement me")
}

// DeleteKeyValues deletes key values from a Vault Secret.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	panic("implement me")
}
