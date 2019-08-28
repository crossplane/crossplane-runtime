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

type MockConditionSetter struct{ Conditions []v1alpha1.Condition }

func (m *MockConditionSetter) SetConditions(c ...v1alpha1.Condition) { m.Conditions = c }

type MockClaimReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClaimReferencer) SetClaimReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockClaimReferencer) GetClaimReference() *corev1.ObjectReference  { return m.Ref }

type MockClassReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockClassReferencer) SetClassReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockClassReferencer) GetClassReference() *corev1.ObjectReference  { return m.Ref }

type MockDefaultClassReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockDefaultClassReferencer) SetDefaultClassReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockDefaultClassReferencer) GetDefaultClassReference() *corev1.ObjectReference  { return m.Ref }

type MockManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

func (m *MockManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }
func (m *MockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference  { return m.Ref }

type MockConnectionSecretWriterTo struct {
	Ref v1alpha1.RequiredLocalObjectReference
}

func (m *MockConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r v1alpha1.RequiredLocalObjectReference) {
	m.Ref = r
}
func (m *MockConnectionSecretWriterTo) GetWriteConnectionSecretToReference() v1alpha1.RequiredLocalObjectReference {
	return m.Ref
}

type MockReclaimer struct{ Policy v1alpha1.ReclaimPolicy }

func (m *MockReclaimer) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) { m.Policy = p }
func (m *MockReclaimer) GetReclaimPolicy() v1alpha1.ReclaimPolicy  { return m.Policy }

var _ Claim = &MockClaim{}

type MockClaim struct {
	runtime.Object

	metav1.ObjectMeta
	MockClassReferencer
	MockManagedResourceReferencer
	MockConnectionSecretWriterTo
	MockConditionSetter
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
	MockConditionSetter
	MockBindable
}

var _ Policy = &MockPolicy{}

type MockPolicy struct {
	runtime.Object

	metav1.ObjectMeta
	MockDefaultClassReferencer
}

var _ PolicyList = &MockPolicyList{}

type MockPolicyList struct {
	runtime.Object

	metav1.ListInterface
}
