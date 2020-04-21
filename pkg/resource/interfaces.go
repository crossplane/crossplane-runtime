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
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A Bindable resource may be bound to another resource. Resources are bindable
// when they available for use.
type Bindable interface {
	SetBindingPhase(p v1alpha1.BindingPhase)
	GetBindingPhase() v1alpha1.BindingPhase
}

// A Conditioned may have conditions set or retrieved. Conditions are typically
// indicate the status of both a resource and its reconciliation process.
type Conditioned interface {
	SetConditions(c ...v1alpha1.Condition)
	GetCondition(v1alpha1.ConditionType) v1alpha1.Condition
}

// A ClaimReferencer may reference a resource claim.
type ClaimReferencer interface {
	SetClaimReference(r *corev1.ObjectReference)
	GetClaimReference() *corev1.ObjectReference
}

// A ClassSelector may reference a resource class.
type ClassSelector interface {
	SetClassSelector(s *metav1.LabelSelector)
	GetClassSelector() *metav1.LabelSelector
}

// A ClassReferencer may reference a resource class.
type ClassReferencer interface {
	SetClassReference(r *corev1.ObjectReference)
	GetClassReference() *corev1.ObjectReference
}

// A ManagedResourceReferencer may reference a concrete managed resource.
type ManagedResourceReferencer interface {
	SetResourceReference(r *corev1.ObjectReference)
	GetResourceReference() *corev1.ObjectReference
}

// A LocalConnectionSecretWriterTo may write a connection secret to its own
// namespace.
type LocalConnectionSecretWriterTo interface {
	SetWriteConnectionSecretToReference(r *v1alpha1.LocalSecretReference)
	GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference
}

// A ConnectionSecretWriterTo may write a connection secret to an arbitrary
// namespace.
type ConnectionSecretWriterTo interface {
	SetWriteConnectionSecretToReference(r *v1alpha1.SecretReference)
	GetWriteConnectionSecretToReference() *v1alpha1.SecretReference
}

// A Reclaimer may specify a ReclaimPolicy.
type Reclaimer interface {
	SetReclaimPolicy(p v1alpha1.ReclaimPolicy)
	GetReclaimPolicy() v1alpha1.ReclaimPolicy
}

// A CredentialsSecretReferencer may refer to a credential secret in an arbitrary
// namespace.
type CredentialsSecretReferencer interface {
	GetCredentialsSecretReference() v1alpha1.SecretKeySelector
	SetCredentialsSecretReference(r v1alpha1.SecretKeySelector)
}

// A ProviderReferencer may reference a provider resource.
type ProviderReferencer interface {
	GetProviderReference() *corev1.ObjectReference
	SetProviderReference(p *corev1.ObjectReference)
}

// A Finalizer manages the finalizers on the resource.
type Finalizer interface {
	AddFinalizer(ctx context.Context, obj Object) error
	RemoveFinalizer(ctx context.Context, obj Object) error
}

// A CompositionSelector may select a composition of resources.
type CompositionSelector interface {
	SetCompositionSelector(*metav1.LabelSelector)
	GetCompositionSelector() *metav1.LabelSelector
}

// A CompositionReferencer may reference a composition of resources.
type CompositionReferencer interface {
	SetCompositionReference(*corev1.ObjectReference)
	GetCompositionReference() *corev1.ObjectReference
}

// A ComposedResourcesReferencer may reference the resources it composes.
type ComposedResourcesReferencer interface {
	SetResourceReferences([]corev1.ObjectReference)
	GetResourceReferences() []corev1.ObjectReference
}

// An Object is a Kubernetes object.
type Object interface {
	metav1.Object
	runtime.Object
}

// A Claim is a Kubernetes object representing an abstract resource claim (e.g.
// an SQL database) that may be bound to a concrete managed resource (e.g. a
// CloudSQL instance).
type Claim interface {
	Object

	ClassSelector
	ClassReferencer
	ManagedResourceReferencer
	LocalConnectionSecretWriterTo

	Conditioned
	Bindable
}

// A ClaimList is a list of resource claims.
type ClaimList interface {
	runtime.Object

	// GetItems returns the list of resource claims.
	GetItems() []Claim
}

// A Class is a Kubernetes object representing configuration specifications for
// a managed resource.
type Class interface {
	Object

	Reclaimer
}

// A ClassList is a list of resource classes.
type ClassList interface {
	runtime.Object

	// GetItems returns the list of resource classes.
	GetItems() []Class
}

// A Managed is a Kubernetes object representing a concrete managed
// resource (e.g. a CloudSQL instance).
type Managed interface {
	Object

	ClassReferencer
	ClaimReferencer
	ProviderReferencer
	ConnectionSecretWriterTo
	Reclaimer

	Conditioned
	Bindable
}

// A ManagedList is a list of managed resources.
type ManagedList interface {
	runtime.Object

	// GetItems returns the list of managed resources.
	GetItems() []Managed
}

// A Provider is a Kubernetes object that refers to credentials to connect
// to an external system.
type Provider interface {
	Object

	CredentialsSecretReferencer
}

// A Target is a Kubernetes object that refers to credentials to connect
// to a deployment target. Target is a subset of the Claim interface.
type Target interface {
	Object

	LocalConnectionSecretWriterTo
	ManagedResourceReferencer

	Conditioned
}

// Composite resource manages one or more Composable resources.
type Composite interface {
	Object

	CompositionSelector
	CompositionReferencer
	ComposedResourcesReferencer
	ConnectionSecretWriterTo

	Conditioned
}

// Composable resources can be a resource in a composition.
type Composable interface {
	Object

	Conditioned
	ConnectionSecretWriterTo
}
