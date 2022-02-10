package fake

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/connection/secret/store"
)

type SecretStore struct {
	ReadKeyValuesFn   func(ctx context.Context, i store.SecretInstance) (store.KeyValues, error)
	WriteKeyValuesFn  func(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error
	DeleteKeyValuesFn func(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error
}

func (ss *SecretStore) ReadKeyValues(ctx context.Context, i store.SecretInstance) (store.KeyValues, error) {
	return ss.ReadKeyValuesFn(ctx, i)
}

func (ss *SecretStore) WriteKeyValues(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error {
	return ss.WriteKeyValuesFn(ctx, i, kv)
}

func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i store.SecretInstance, kv store.KeyValues) error {
	return ss.DeleteKeyValuesFn(ctx, i, kv)
}
