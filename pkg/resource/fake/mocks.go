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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// Bindable is a mock that implements Bindable interface.
type Bindable struct{ Phase v1alpha1.BindingPhase }

// SetBindingPhase sets the BindingPhase.
func (m *Bindable) SetBindingPhase(p v1alpha1.BindingPhase) { m.Phase = p }

// GetBindingPhase sets the BindingPhase.
func (m *Bindable) GetBindingPhase() v1alpha1.BindingPhase { return m.Phase }

// Conditioned is a mock that implements Conditioned interface.
type Conditioned struct{ Conditions []v1alpha1.Condition }

// SetConditions sets the Conditions.
func (m *Conditioned) SetConditions(c ...v1alpha1.Condition) { m.Conditions = c }

// GetCondition get the Condition with the given ConditionType.
func (m *Conditioned) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	return v1alpha1.Condition{Type: ct, Status: corev1.ConditionUnknown}
}

// ClaimReferencer is a mock that implements ClaimReferencer interface.
type ClaimReferencer struct{ Ref *corev1.ObjectReference }

// SetClaimReference sets the ClaimReference.
func (m *ClaimReferencer) SetClaimReference(r *corev1.ObjectReference) { m.Ref = r }

// GetClaimReference gets the ClaimReference.
func (m *ClaimReferencer) GetClaimReference() *corev1.ObjectReference { return m.Ref }

// ClassSelector is a mock that implements ClassSelector interface.
type ClassSelector struct{ Sel *metav1.LabelSelector }

// SetClassSelector sets the ClassSelector.
func (m *ClassSelector) SetClassSelector(s *metav1.LabelSelector) {
	m.Sel = s
}

// GetClassSelector gets the ClassSelector.
func (m *ClassSelector) GetClassSelector() *metav1.LabelSelector {
	return m.Sel
}

// ClassReferencer is a mock that implements ClassReferencer interface.
type ClassReferencer struct{ Ref *corev1.ObjectReference }

// SetClassReference sets the ClassReference.
func (m *ClassReferencer) SetClassReference(r *corev1.ObjectReference) {
	m.Ref = r
}

// GetClassReference gets the ClassReference.
func (m *ClassReferencer) GetClassReference() *corev1.ObjectReference {
	return m.Ref
}

// ManagedResourceReferencer is a mock that implements ManagedResourceReferencer interface.
type ManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

// SetResourceReference sets the ResourceReference.
func (m *ManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }

// GetResourceReference gets the ResourceReference.
func (m *ManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference { return m.Ref }

// LocalConnectionSecretWriterTo is a mock that implements LocalConnectionSecretWriterTo interface.
type LocalConnectionSecretWriterTo struct {
	Ref *v1alpha1.LocalSecretReference
}

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *LocalConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *LocalConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	return m.Ref
}

// ConnectionSecretWriterTo is a mock that implements ConnectionSecretWriterTo interface.
type ConnectionSecretWriterTo struct{ Ref *v1alpha1.SecretReference }

// SetWriteConnectionSecretToReference sets the WriteConnectionSecretToReference.
func (m *ConnectionSecretWriterTo) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	m.Ref = r
}

// GetWriteConnectionSecretToReference gets the WriteConnectionSecretToReference.
func (m *ConnectionSecretWriterTo) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	return m.Ref
}

// Reclaimer is a mock that implements Reclaimer interface.
type Reclaimer struct{ Policy v1alpha1.ReclaimPolicy }

// SetReclaimPolicy sets the ReclaimPolicy.
func (m *Reclaimer) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) { m.Policy = p }

// GetReclaimPolicy gets the ReclaimPolicy.
func (m *Reclaimer) GetReclaimPolicy() v1alpha1.ReclaimPolicy { return m.Policy }

// CredentialsSecretReferencer is a mock that satisfies CredentialsSecretReferencer
// interface.
type CredentialsSecretReferencer struct{ Ref v1alpha1.SecretKeySelector }

// SetCredentialsSecretReference sets CredentialsSecretReference.
func (m *CredentialsSecretReferencer) SetCredentialsSecretReference(r v1alpha1.SecretKeySelector) {
	m.Ref = r
}

// GetCredentialsSecretReference gets CredentialsSecretReference.
func (m *CredentialsSecretReferencer) GetCredentialsSecretReference() v1alpha1.SecretKeySelector {
	return m.Ref
}

// Claim is a mock that implements Claim interface.
type Claim struct {
	metav1.ObjectMeta
	ClassSelector
	ClassReferencer
	ManagedResourceReferencer
	LocalConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
	v1alpha1.BindingStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Claim) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Claim) DeepCopyObject() runtime.Object {
	out := &Claim{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Class is a mock that implements Class interface.
type Class struct {
	metav1.ObjectMeta
	Reclaimer
}

// GetObjectKind returns schema.ObjectKind.
func (m *Class) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Class) DeepCopyObject() runtime.Object {
	out := &Class{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Managed is a mock that implements Managed interface.
type Managed struct {
	metav1.ObjectMeta
	ClassReferencer
	ClaimReferencer
	ConnectionSecretWriterTo
	Reclaimer
	v1alpha1.ConditionedStatus
	v1alpha1.BindingStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Managed) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Managed) DeepCopyObject() runtime.Object {
	out := &Managed{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Provider is a mock that satisfies Provider interface.
type Provider struct {
	metav1.ObjectMeta
	CredentialsSecretReferencer
}

// GetObjectKind returns schema.ObjectKind.
func (m *Provider) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a deep copy of Provider as runtime.Object.
func (m *Provider) DeepCopyObject() runtime.Object {
	out := &Provider{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Target is a mock that implements Target interface.
type Target struct {
	metav1.ObjectMeta
	ManagedResourceReferencer
	LocalConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Target) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a deep copy of Target as runtime.Object.
func (m *Target) DeepCopyObject() runtime.Object {
	out := &Target{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Manager is a mock object that satisfies manager.Manager interface.
type Manager struct {
	manager.Manager

	Client client.Client
	Scheme *runtime.Scheme
}

// GetClient returns the client.
func (m *Manager) GetClient() client.Client { return m.Client }

// GetScheme returns the scheme.
func (m *Manager) GetScheme() *runtime.Scheme { return m.Scheme }

// GV returns a mock schema.GroupVersion.
var GV = schema.GroupVersion{Group: "g", Version: "v"}

// GVK returns the mock GVK of the given object.
func GVK(o runtime.Object) schema.GroupVersionKind {
	return GV.WithKind(reflect.TypeOf(o).Elem().Name())
}

// SchemeWith returns a scheme with list of `runtime.Object`s registered.
func SchemeWith(o ...runtime.Object) *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypes(GV, o...)
	return s
}
