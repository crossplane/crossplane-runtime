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

// Package fake provides fake Crossplane resources for use in tests.
package fake

import (
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
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
func (m *ClassSelector) SetClassSelector(s *metav1.LabelSelector) { m.Sel = s }

// GetClassSelector gets the ClassSelector.
func (m *ClassSelector) GetClassSelector() *metav1.LabelSelector { return m.Sel }

// ClassReferencer is a mock that implements ClassReferencer interface.
type ClassReferencer struct{ Ref *corev1.ObjectReference }

// SetClassReference sets the ClassReference.
func (m *ClassReferencer) SetClassReference(r *corev1.ObjectReference) { m.Ref = r }

// GetClassReference gets the ClassReference.
func (m *ClassReferencer) GetClassReference() *corev1.ObjectReference { return m.Ref }

// ManagedResourceReferencer is a mock that implements ManagedResourceReferencer interface.
type ManagedResourceReferencer struct{ Ref *corev1.ObjectReference }

// SetResourceReference sets the ResourceReference.
func (m *ManagedResourceReferencer) SetResourceReference(r *corev1.ObjectReference) { m.Ref = r }

// GetResourceReference gets the ResourceReference.
func (m *ManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference { return m.Ref }

// ProviderReferencer is a mock that implements ProviderReferencer interface.
type ProviderReferencer struct{ Ref *corev1.ObjectReference }

// SetProviderReference sets the ProviderReference.
func (m *ProviderReferencer) SetProviderReference(p *corev1.ObjectReference) { m.Ref = p }

// GetProviderReference gets the ProviderReference.
func (m *ProviderReferencer) GetProviderReference() *corev1.ObjectReference { return m.Ref }

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

// CompositionReferencer is a mock that implements CompositionReferencer interface.
type CompositionReferencer struct{ Ref *corev1.ObjectReference }

// SetCompositionReference sets the CompositionReference.
func (m *CompositionReferencer) SetCompositionReference(r *corev1.ObjectReference) { m.Ref = r }

// GetCompositionReference gets the CompositionReference.
func (m *CompositionReferencer) GetCompositionReference() *corev1.ObjectReference { return m.Ref }

// CompositionSelector is a mock that implements CompositionSelector interface.
type CompositionSelector struct{ Sel *metav1.LabelSelector }

// SetCompositionSelector sets the CompositionSelector.
func (m *CompositionSelector) SetCompositionSelector(s *metav1.LabelSelector) { m.Sel = s }

// GetCompositionSelector gets the CompositionSelector.
func (m *CompositionSelector) GetCompositionSelector() *metav1.LabelSelector { return m.Sel }

// CompositeResourceReferencer is a mock that implements CompositeResourceReferencer interface.
type CompositeResourceReferencer struct{ Ref *corev1.ObjectReference }

// SetResourceReference sets the composite resource reference.
func (m *CompositeResourceReferencer) SetResourceReference(p *corev1.ObjectReference) { m.Ref = p }

// GetResourceReference gets the composite resource reference.
func (m *CompositeResourceReferencer) GetResourceReference() *corev1.ObjectReference { return m.Ref }

// RequirementReferencer is a mock that implements RequirementReferencer interface.
type RequirementReferencer struct{ Ref *corev1.ObjectReference }

// SetRequirementReference sets the requirement reference.
func (m *RequirementReferencer) SetRequirementReference(p *corev1.ObjectReference) { m.Ref = p }

// GetRequirementReference gets the requirement reference.
func (m *RequirementReferencer) GetRequirementReference() *corev1.ObjectReference { return m.Ref }

// ComposedResourcesReferencer is a mock that implements ComposedResourcesReferencer interface.
type ComposedResourcesReferencer struct{ Refs []corev1.ObjectReference }

// SetResourceReferences sets the composed references.
func (m *ComposedResourcesReferencer) SetResourceReferences(r []corev1.ObjectReference) { m.Refs = r }

// GetResourceReferences gets the composed references.
func (m *ComposedResourcesReferencer) GetResourceReferences() []corev1.ObjectReference { return m.Refs }

// Object is a mock that implements Object interface.
type Object struct {
	metav1.ObjectMeta
	runtime.Object
}

// GetObjectKind returns schema.ObjectKind.
func (o *Object) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (o *Object) DeepCopyObject() runtime.Object {
	out := &Object{}
	j, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
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
	ProviderReferencer
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

// Composite is a mock that implements Composite interface.
type Composite struct {
	metav1.ObjectMeta
	CompositionSelector
	CompositionReferencer
	ComposedResourcesReferencer
	RequirementReferencer
	Reclaimer
	ConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composite) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Composite) DeepCopyObject() runtime.Object {
	out := &Composite{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Composed is a mock that implements Composed interface.
type Composed struct {
	metav1.ObjectMeta
	ConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composed) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Composed) DeepCopyObject() runtime.Object {
	out := &Composed{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Requirement is a mock that implements Requirement interface.
type Requirement struct {
	metav1.ObjectMeta
	CompositionSelector
	CompositionReferencer
	CompositeResourceReferencer
	LocalConnectionSecretWriterTo
	v1alpha1.ConditionedStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Requirement) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *Requirement) DeepCopyObject() runtime.Object {
	out := &Requirement{}
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

	Client     client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	RESTMapper meta.RESTMapper
}

// Elected returns a closed channel.
func (m *Manager) Elected() <-chan struct{} {
	e := make(chan struct{})
	close(e)
	return e
}

// GetClient returns the client.
func (m *Manager) GetClient() client.Client { return m.Client }

// GetScheme returns the scheme.
func (m *Manager) GetScheme() *runtime.Scheme { return m.Scheme }

// GetConfig returns the config.
func (m *Manager) GetConfig() *rest.Config { return m.Config }

// GetRESTMapper returns the REST mapper.
func (m *Manager) GetRESTMapper() meta.RESTMapper { return m.RESTMapper }

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

// MockConnectionSecretOwner is a mock object that satisfies ConnectionSecretOwner
// interface.
type MockConnectionSecretOwner struct {
	runtime.Object
	metav1.ObjectMeta

	Ref *v1alpha1.SecretReference
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (m *MockConnectionSecretOwner) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	return m.Ref
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (m *MockConnectionSecretOwner) SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference) {
	m.Ref = r
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockConnectionSecretOwner) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *MockConnectionSecretOwner) DeepCopyObject() runtime.Object {
	out := &MockConnectionSecretOwner{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// MockLocalConnectionSecretOwner is a mock object that satisfies LocalConnectionSecretOwner
// interface.
type MockLocalConnectionSecretOwner struct {
	runtime.Object
	metav1.ObjectMeta

	Ref *v1alpha1.LocalSecretReference
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (m *MockLocalConnectionSecretOwner) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	return m.Ref
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (m *MockLocalConnectionSecretOwner) SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference) {
	m.Ref = r
}

// GetObjectKind returns schema.ObjectKind.
func (m *MockLocalConnectionSecretOwner) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object
func (m *MockLocalConnectionSecretOwner) DeepCopyObject() runtime.Object {
	out := &MockLocalConnectionSecretOwner{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}
