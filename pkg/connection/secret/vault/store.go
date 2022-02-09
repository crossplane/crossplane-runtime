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
