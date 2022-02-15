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

package fake

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
)

// SecretStore is a fake SecretStore
type SecretStore struct {
	ReadKeyValuesFn   func(ctx context.Context, i store.Secret) (store.KeyValues, error)
	WriteKeyValuesFn  func(ctx context.Context, i store.Secret, kv store.KeyValues) error
	DeleteKeyValuesFn func(ctx context.Context, i store.Secret, kv store.KeyValues) error
}

// ReadKeyValues reads key values.
func (ss *SecretStore) ReadKeyValues(ctx context.Context, i store.Secret) (store.KeyValues, error) {
	return ss.ReadKeyValuesFn(ctx, i)
}

// WriteKeyValues writes key values.
func (ss *SecretStore) WriteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	return ss.WriteKeyValuesFn(ctx, i, kv)
}

// DeleteKeyValues deletes key values.
func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error {
	return ss.DeleteKeyValuesFn(ctx, i, kv)
}
