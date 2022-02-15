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

package connection

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/connection/store"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// A DetailsPublisherTo may write a connection details secret to a secret store
type DetailsPublisherTo interface {
	SetPublishConnectionDetailsTo(r *v1.PublishConnectionDetailsTo)
	GetPublishConnectionDetailsTo() *v1.PublishConnectionDetailsTo
}

// A SecretOwner is a Kubernetes object that owns a connection secret.
type SecretOwner interface {
	runtime.Object
	metav1.Object

	DetailsPublisherTo
}

// A StoreConfig configures a connection store.
type StoreConfig interface {
	resource.Object

	resource.Conditioned
	GetStoreConfig() v1.SecretStoreConfig
}

// A Store stores sensitive key values in Secret.
type Store interface {
	ReadKeyValues(ctx context.Context, i store.Secret) (store.KeyValues, error)
	WriteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error
	DeleteKeyValues(ctx context.Context, i store.Secret, kv store.KeyValues) error
}
