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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
)

// MockSecretOwner is a mock object that satisfies ConnectionSecretOwner
// interface.
type MockSecretOwner struct {
	runtime.Object
	metav1.ObjectMeta

	To *v1.PublishConnectionDetailsTo
}

// GetPublishConnectionDetailsTo returns the publish connection details to reference.
func (m *MockSecretOwner) GetPublishConnectionDetailsTo() *v1.PublishConnectionDetailsTo {
	return m.To
}

// SetPublishConnectionDetailsTo sets the publish connection details to reference.
func (m *MockSecretOwner) SetPublishConnectionDetailsTo(t *v1.PublishConnectionDetailsTo) {
	m.To = t
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockSecretOwner) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *MockSecretOwner) DeepCopyObject() runtime.Object {
	out := &MockSecretOwner{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

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
