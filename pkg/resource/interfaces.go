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

// An Orphanable resource may specify a DeletionPolicy.
type Orphanable interface {
	SetDeletionPolicy(p v1alpha1.DeletionPolicy)
	GetDeletionPolicy() v1alpha1.DeletionPolicy
}

// A ProviderReferencer may reference a provider resource.
type ProviderReferencer interface {
	GetProviderReference() *v1alpha1.Reference
	SetProviderReference(p *v1alpha1.Reference)
}

// A ProviderConfigReferencer may reference a provider config resource.
type ProviderConfigReferencer interface {
	GetProviderConfigReference() *v1alpha1.Reference
	SetProviderConfigReference(p *v1alpha1.Reference)
}

// A RequiredProviderConfigReferencer may reference a provider config resource.
// Unlike ProviderConfigReferencer, the reference is required (i.e. not nil).
type RequiredProviderConfigReferencer interface {
	GetProviderConfigReference() v1alpha1.Reference
	SetProviderConfigReference(p v1alpha1.Reference)
}

// A RequiredTypedResourceReferencer can reference a resource.
type RequiredTypedResourceReferencer interface {
	SetResourceReference(r v1alpha1.TypedReference)
	GetResourceReference() v1alpha1.TypedReference
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

// A CompositeResourceReferencer can reference a composite resource.
type CompositeResourceReferencer interface {
	SetResourceReference(r *corev1.ObjectReference)
	GetResourceReference() *corev1.ObjectReference
}

// A UserCounter can count how many users it has.
type UserCounter interface {
	SetUsers(i int64)
	GetUsers() int64
}

// An Object is a Kubernetes object.
type Object interface {
	metav1.Object
	runtime.Object
}

// A Managed is a Kubernetes object representing a concrete managed
// resource (e.g. a CloudSQL instance).
type Managed interface {
	Object

	ProviderReferencer
	ProviderConfigReferencer
	ConnectionSecretWriterTo
	Orphanable

	Conditioned
}

// A ManagedList is a list of managed resources.
type ManagedList interface {
	runtime.Object

	// GetItems returns the list of managed resources.
	GetItems() []Managed
}

// A ProviderConfig configures a Crossplane provider.
type ProviderConfig interface {
	Object

	UserCounter
	Conditioned
}

// A ProviderConfigUsage indicates a usage of a Crossplane provider config.
type ProviderConfigUsage interface {
	Object

	RequiredProviderConfigReferencer
	RequiredTypedResourceReferencer
}

// A ProviderConfigUsageList is a list of provider config usages.
type ProviderConfigUsageList interface {
	runtime.Object

	// GetItems returns the list of provider config usages.
	GetItems() []ProviderConfigUsage
}

// A Target is a Kubernetes object that refers to credentials to connect
// to a deployment target. Target is a subset of the Claim interface.
type Target interface {
	Object

	LocalConnectionSecretWriterTo
	ManagedResourceReferencer

	Conditioned
}

// A Composite resource composes one or more Composed resources.
type Composite interface {
	Object

	CompositionSelector
	CompositionReferencer
	ComposedResourcesReferencer
	ClaimReferencer
	ConnectionSecretWriterTo

	Conditioned
}

// Composed resources can be a composed into a Composite resource.
type Composed interface {
	Object

	Conditioned
	ConnectionSecretWriterTo
}

// A CompositeClaim for a Composite resource.
type CompositeClaim interface {
	Object

	CompositionSelector
	CompositionReferencer
	CompositeResourceReferencer
	LocalConnectionSecretWriterTo

	Conditioned
}
