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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

type MockBindable struct{ Phase v1alpha1.BindingPhase }

func (m *MockBindable) SetBindingPhase(p v1alpha1.BindingPhase) { m.Phase = p }
func (m *MockBindable) GetBindingPhase() v1alpha1.BindingPhase  { return m.Phase }

type MockConditioned struct{ Conditions []v1alpha1.Condition }

func (m *MockConditioned) SetConditions(c ...v1alpha1.Condition) { m.Conditions = c }
func (m *MockConditioned) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	return v1alpha1.Condition{Type: ct, Status: corev1.ConditionUnknown}
}

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

var _ Claim = &MockClaim{}

type MockClaim struct {
	runtime.Object

	metav1.ObjectMeta
	MockClassSelector
	MockClassReferencer
	MockManagedResourceReferencer
	MockLocalConnectionSecretWriterTo
	MockConditioned
	MockBindable
}

var _ Class = &MockClass{}

type MockClass struct {
	runtime.Object

	metav1.ObjectMeta
	MockReclaimer
}

var _ Managed = &MockManaged{}

type MockManaged struct {
	runtime.Object

	metav1.ObjectMeta
	MockClassReferencer
	MockClaimReferencer
	MockConnectionSecretWriterTo
	MockReclaimer
	MockConditioned
	MockBindable
}
