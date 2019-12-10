/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

type MockClaimReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClaimReferencer) SetClaimReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockClaimReferencer) GetClaimReference() *corev1.ObjectReference  { return m.Ref }

type MockClassSelector struct{ Sel *metav1.LabelSelector }

func (m *MockClassSelector) SetClassSelector(s *metav1.LabelSelector) {
	m.Sel = s
}
func (m *MockClassSelector) GetClassSelector() *metav1.LabelSelector {
	return m.Sel
}

type MockClassReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClassReferencer) SetClassReference(r *corev1.ObjectReference) {
	m.Ref = r
}
func (m *MockClassReferencer) GetClassReference() *corev1.ObjectReference {
	return m.Ref
}

type MockManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference  { return m.Ref }

type MockLocalConnectionSecretWriterTo struct {
	Ref *v1alpha1.LocalSecretReference
}

func (m *MockLocalConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference) {
	m.Ref = r
}
func (m *MockLocalConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	return m.Ref
}

type MockConnectionSecretWriterTo struct{ Ref *v1alpha1.SecretReference }

func (m *MockConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	m.Ref = r
}
func (m *MockConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	return m.Ref
}

type MockReclaimer struct{ Policy v1alpha1.ReclaimPolicy }

func (m *MockReclaimer) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) { m.Policy = p }
func (m *MockReclaimer) GetReclaimPolicy() v1alpha1.ReclaimPolicy  { return m.Policy }

type MockCredentialsSecretReferencer struct{ Ref *v1alpha1.SecretKeySelector }

func (m *MockCredentialsSecretReferencer) SetCredentialsSecretReference(r *v1alpha1.SecretKeySelector) {
	m.Ref = r
}
func (m *MockCredentialsSecretReferencer) GetCredentialsSecretReference() *v1alpha1.SecretKeySelector {
	return m.Ref
}

var _ Claim = &MockClaim{}

type MockClaim struct {
	metav1.ObjectMeta
	MockClassSelector
	MockClassReferencer
	MockManagedResourceReferencer
	MockLocalConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
	v1alpha1.BindingStatus
}

func (m *MockClaim) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *MockClaim) DeepCopyObject() runtime.Object {
	out := &MockClaim{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(j, out)
	return out
}

var _ Class = &MockClass{}

type MockClass struct {
	metav1.ObjectMeta
	MockReclaimer
}

func (m *MockClass) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *MockClass) DeepCopyObject() runtime.Object {
	out := &MockClass{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(j, out)
	return out
}

var _ Managed = &MockManaged{}

type MockManaged struct {
	metav1.ObjectMeta
	MockClassReferencer
	MockClaimReferencer
	MockConnectionSecretWriterTo
	MockReclaimer
	v1alpha1.ConditionedStatus
	v1alpha1.BindingStatus
}

func (m *MockManaged) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *MockManaged) DeepCopyObject() runtime.Object {
	out := &MockManaged{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(j, out)
	return out
}

var _ Provider = &MockProvider{}

type MockProvider struct {
	metav1.ObjectMeta
	MockCredentialsSecretReferencer
}

func (m *MockProvider) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (m *MockProvider) DeepCopyObject() runtime.Object {
	out := &MockProvider{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(j, out)
	return out
}
