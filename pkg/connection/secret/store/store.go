package store

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type KeyValues map[string][]byte

type SecretInstance struct {
	Name     string
	Scope    string
	Owner    metav1.OwnerReference
	Metadata v1.ConnectionSecretMetadata
}

type KeyValuesReader interface {
	ReadKeyValues(ctx context.Context, i SecretInstance) (KeyValues, error)
}

type KeyValuesWriter interface {
	WriteKeyValues(ctx context.Context, i SecretInstance, kv KeyValues) error
}

type KeyValuesDeleter interface {
	DeleteKeyValues(ctx context.Context, i SecretInstance, kv KeyValues) error
}

type Store interface {
	KeyValuesReader
	KeyValuesWriter
	KeyValuesDeleter
}
