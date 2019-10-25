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

package fake

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// MockBindable is a mock that implements Bindable interface.
type MockBindable struct{ Phase v1alpha1.BindingPhase }

// SetBindingPhase sets the BindingPhase.
func (m *MockBindable) SetBindingPhase(p v1alpha1.BindingPhase) { m.Phase = p }

// GetBindingPhase sets the BindingPhase.
func (m *MockBindable) GetBindingPhase() v1alpha1.BindingPhase { return m.Phase }

// MockConditioned is a mock that implements Conditioned interface.
type MockConditioned struct{ Conditions []v1alpha1.Condition }

// SetConditions sets the Conditions.
func (m *MockConditioned) SetConditions(c ...v1alpha1.Condition) { m.Conditions = c }

// GetCondition get the Condition with the given ConditionType.
func (m *MockConditioned) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	return v1alpha1.Condition{Type: ct, Status: corev1.ConditionUnknown}
}

// MockClaimReferencer is a mock that implements ClaimReferencer interface.
type MockClaimReferencer struct{ Ref *corev1.ObjectReference }

// SetClaimReference sets the ClaimReference.
func (m *MockClaimReferencer) SetClaimReference(r *corev1.ObjectReference) { m.Ref = r }

// GetClaimReference gets the ClaimReference.
func (m *MockClaimReferencer) GetClaimReference() *corev1.ObjectReference { return m.Ref }

// MockClassSelector is a mock that implements ClassSelector interface.
type MockClassSelector struct{ Sel *metav1.LabelSelector }

// SetClassSelector sets the ClassSelector.
func (m *MockClassSelector) SetClassSelector(s *metav1.LabelSelector) {
	m.Sel = s
}

// GetClassSelector gets the ClassSelector.
func (m *MockClassSelector) GetClassSelector() *metav1.LabelSelector {
	return m.Sel
}

// MockClassReferencer is a mock that implements ClassReferencer interface.
type MockClassReferencer struct{ Ref *corev1.ObjectReference }

// SetClassReference sets the ClassReference.
func (m *MockClassReferencer) SetClassReference(r *corev1.ObjectReference) {
	m.Ref = r
}

// GetClassReference gets the ClassReference.
func (m *MockClassReferencer) GetClassReference() *corev1.ObjectReference {
	return m.Ref
}

// MockManagedResourceReferencer is a mock that implements ManagedResourceReferencer interface.
type MockManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

// SetResourceReference sets the ResourceReference.
func (m *MockManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }

// GetResourceReference gets the ResourceReference.
func (m *MockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference { return m.Ref }

// MockLocalConnectionSecretWriterTo is a mock that implements LocalConnectionSecretWriterTo interface.
type MockLocalConnectionSecretWriterTo struct {
	Ref *v1alpha1.LocalSecretReference
}

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *MockLocalConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *MockLocalConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	return m.Ref
}

// MockConnectionSecretWriterTo is a mock that implements ConnectionSecretWriterTo interface.
type MockConnectionSecretWriterTo struct{ Ref *v1alpha1.SecretReference }

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *MockConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *MockConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	return m.Ref
}

// MockReclaimer is a mock that implements Reclaimer interface.
type MockReclaimer struct{ Policy v1alpha1.ReclaimPolicy }

// SetReclaimPolicy sets the ReclaimPolicy.
func (m *MockReclaimer) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) { m.Policy = p }

// MockCredentialsSecretReferencer is a mock that satisfies CredentialsSecretReferencer
// interface.
type MockCredentialsSecretReferencer struct{ Ref *v1alpha1.SecretKeySelector }

// SetCredentialsSecretReference sets CredentialsSecretReference.
func (m *MockCredentialsSecretReferencer) SetCredentialsSecretReference(r *v1alpha1.SecretKeySelector) {
	m.Ref = r
}

// GetCredentialsSecretReference gets CredentialsSecretReference.
func (m *MockCredentialsSecretReferencer) GetCredentialsSecretReference() *v1alpha1.SecretKeySelector {
	return m.Ref
}

// GetReclaimPolicy gets the ReclaimPolicy.
func (m *MockReclaimer) GetReclaimPolicy() v1alpha1.ReclaimPolicy { return m.Policy }

// MockClaim is a mock that implements Claim interface.
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

// MockClass is a mock that implements Class interface.
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

// MockManaged is a mock that implements Managed interface.
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

// MockProvider is a mock that satisfies Provider interface.
type MockProvider struct {
	metav1.ObjectMeta
	MockCredentialsSecretReferencer
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockProvider) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// GetObjectKind returns a deep copy of MockProvider as runtime.Object.
func (m *MockProvider) DeepCopyObject() runtime.Object {
	out := &MockProvider{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(j, out)
	return out
}
