package vault

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/connection/secret"
)

type SecretStore struct{}

func NewSecretStore() *SecretStore {
	return &SecretStore{}
}

func (ss *SecretStore) ReadKeyValues(ctx context.Context, i secret.Instance) (secret.KeyValues, error) {
	panic("implement me")
}

func (ss *SecretStore) WriteKeyValues(ctx context.Context, i secret.Instance, kv secret.KeyValues) error {
	panic("implement me")
}

func (ss *SecretStore) DeleteKeyValues(ctx context.Context, i secret.Instance, kv secret.KeyValues) error {
	panic("implement me")
}
