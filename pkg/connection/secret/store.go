package secret

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type KeyValues map[string][]byte
type Instance struct {
	Name     string
	Scope    string
	Owner    metav1.OwnerReference
	Metadata v1.ConnectionSecretMetadata
}

type KeyValuesReader interface {
	ReadKeyValues(ctx context.Context, i Instance) (KeyValues, error)
}

type KeyValuesWriter interface {
	WriteKeyValues(ctx context.Context, i Instance, kv KeyValues) error
}

type KeyValuesDeleter interface {
	DeleteKeyValues(ctx context.Context, i Instance, kv KeyValues) error
}

type Store interface {
	KeyValuesReader
	KeyValuesWriter
	KeyValuesDeleter
}
