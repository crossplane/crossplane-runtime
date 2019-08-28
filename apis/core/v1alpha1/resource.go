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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// ResourceCredentialsSecretEndpointKey is the key inside a connection secret for the connection endpoint
	ResourceCredentialsSecretEndpointKey = "endpoint"
	// ResourceCredentialsSecretUserKey is the key inside a connection secret for the connection user
	ResourceCredentialsSecretUserKey = "username"
	// ResourceCredentialsSecretPasswordKey is the key inside a connection secret for the connection password
	ResourceCredentialsSecretPasswordKey = "password"
	// ResourceCredentialsSecretCAKey is the key inside a connection secret for the server CA certificate
	ResourceCredentialsSecretCAKey = "clusterCA"
	// ResourceCredentialsSecretClientCertKey is the key inside a connection secret for the client certificate
	ResourceCredentialsSecretClientCertKey = "clientCert"
	// ResourceCredentialsSecretClientKeyKey is the key inside a connection secret for the client key
	ResourceCredentialsSecretClientKeyKey = "clientKey"
	// ResourceCredentialsTokenKey is the key inside a connection secret for the bearer token value
	ResourceCredentialsTokenKey = "token"
)

// A RequiredLocalObjectReference contains enough information to let you locate
// the referenced object inside the same namespace. It is identical to
// corev1.LocalObjectReference, but requires the object's Name be specified.
type RequiredLocalObjectReference struct {
	Name string `json:"name"`
}

// ResourceClaimSpec contains standard fields that all resource claims should
// include in their spec. Unlike ResourceClaimStatus, ResourceClaimSpec should
// typically be embedded in a claim specific struct.
type ResourceClaimSpec struct {
	WriteConnectionSecretToReference RequiredLocalObjectReference `json:"writeConnectionSecretToRef"`

	// TODO(negz): Make the below references immutable once set? Doing so means
	// we don't have to track what provisioner was used to create a resource.

	ClassReference    *corev1.ObjectReference `json:"classRef,omitempty"`
	ResourceReference *corev1.ObjectReference `json:"resourceRef,omitempty"`
}

// ResourceClaimStatus represents the status of a resource claim. Claims should
// typically use this struct as their status.
type ResourceClaimStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// ResourceSpec contains standard fields that all resources should
// include in their spec. ResourceSpec should typically be embedded in a
// resource specific struct.
type ResourceSpec struct {
	WriteConnectionSecretToReference RequiredLocalObjectReference `json:"writeConnectionSecretToRef"`

	ClaimReference    *corev1.ObjectReference `json:"claimRef,omitempty"`
	ClassReference    *corev1.ObjectReference `json:"classRef,omitempty"`
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ResourceClassSpecTemplate contains standard fields that all resource classes should
// include in their spec template. ResourceClassSpecTemplate should typically be embedded in a
// resource class specific struct.
type ResourceClassSpecTemplate struct {
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ResourceStatus contains standard fields that all resources should
// include in their status. ResourceStatus should typically be embedded in a
// resource specific status.
type ResourceStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// Policy contains standard fields that all policies should include. Policy
// should typically be embedded in a specific resource claim policy.
type Policy struct {
	DefaultClassReference *corev1.ObjectReference `json:"defaultClassRef,omitempty"`
}

// SetDefaultClassReference of this Policy
func (p *Policy) SetDefaultClassReference(r *corev1.ObjectReference) {
	p.DefaultClassReference = r
}

// GetDefaultClassReference of this Policy
func (p *Policy) GetDefaultClassReference() *corev1.ObjectReference {
	return p.DefaultClassReference
}
