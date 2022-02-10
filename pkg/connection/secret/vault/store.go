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
	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/store"
)

type SecretStore struct{}

func NewSecretStore(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (store.Store, error) {
	return &SecretStore{}, nil
}

func (ss *SecretStore) ReadKeyValues(ctx context.Context, i store.SecretInstance) (store.KeyValues, error) {
	panic("implement me")
}

func (ss *SecretStore) WriteKeyValues(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error {
	panic("implement me")
}

func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error {
	panic("implement me")
}
