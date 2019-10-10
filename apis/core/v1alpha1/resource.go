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
	// ResourceCredentialsSecretPortKey is the key inside a connection secret for the connection port
	ResourceCredentialsSecretPortKey = "port"
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

// A ResourceClaimSpec defines the desired state of a resource claim.
type ResourceClaimSpec struct {
	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this resource claim, to which any connection details
	// for this resource claim should be written. Connection details frequently
	// include the endpoint, username, and password required to connect to the
	// managed resource bound to this resource claim.
	// +optional
	WriteConnectionSecretToReference corev1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`

	// TODO(negz): Make the below references immutable once set? Doing so means
	// we don't have to track what provisioner was used to create a resource.

	// A ClassReference specifies a resource class that will be used to
	// dynamically provision a managed resource when the resource claim is
	// created. Omit the class reference if you wish to bind to a specific
	// managed resource (known as static binding).
	// +optional
	ClassReference *corev1.ObjectReference `json:"classRef,omitempty"`

	// A ResourceReference specifies an existing managed resource, in any
	// namespace, to which this resource claim should attempt to bind. Omit the
	// resource reference to enable dynamic provisioning using a resource class;
	// the resource reference will be automatically populated by Crossplane.
	// +optional
	ResourceReference *corev1.ObjectReference `json:"resourceRef,omitempty"`
}

// A ResourceClaimStatus represents the observed status of a resource claim.
type ResourceClaimStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// TODO(negz): Rename Resource* to Managed* to clarify that they enable the
// resource.Managed interface.

// A ResourceSpec defines the desired state of a managed resource.
type ResourceSpec struct {
	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this managed resource, to which any connection details
	// for this managed resource should be written. Connection details
	// frequently include the endpoint, username, and password required to
	// connect to the managed resource.
	// +optional
	WriteConnectionSecretToReference corev1.LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`

	// ClaimReference specifies the resource claim to which this managed
	// resource will be bound. ClaimReference is set automatically during
	// dynamic provisioning. Crossplane does not currently support setting this
	// field manually, per https://github.com/crossplaneio/crossplane-runtime/issues/19
	// +optional
	ClaimReference *corev1.ObjectReference `json:"claimRef,omitempty"`

	// ClassReference specifies the resource class that was used to dynamically
	// provision this managed resource, if any. Crossplane does not currently
	// support setting this field manually, per
	// https://github.com/crossplaneio/crossplane-runtime/issues/20
	// +optional
	ClassReference *corev1.ObjectReference `json:"classRef,omitempty"`

	// ProviderReference specifies the provider that will be used to create,
	// observe, update, and delete this managed resource.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// ReclaimPolicy specifies what will happen to the external resource this
	// managed resource manages when the managed resource is deleted. "Delete"
	// deletes the external resource, while "Retain" (the default) does not.
	// Note this behaviour is subtly different from other uses of the
	// ReclaimPolicy concept within the Kubernetes ecosystem per
	// https://github.com/crossplaneio/crossplane-runtime/issues/21
	// +optional
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// ResourceStatus represents the observed state of a managed resource.
type ResourceStatus struct {
	ConditionedStatus `json:",inline"`
	BindingStatus     `json:",inline"`
}

// A ClassSpecTemplate defines a template that will be used to create the
// specifications of managed resources dynamically provisioned using a resource
// class.
type ClassSpecTemplate struct {
	// ProviderReference specifies the provider that will be used to create,
	// observe, update, and delete managed resources that are dynamically
	// provisioned using this resource class.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// ReclaimPolicy specifies what will happen to external resources when
	// managed resources dynamically provisioned using this resource class are
	// deleted. "Delete" deletes the external resource, while "Retain" (the
	// default) does not. Note this behaviour is subtly different from other
	// uses of the ReclaimPolicy concept within the Kubernetes ecosystem per
	// https://github.com/crossplaneio/crossplane-runtime/issues/21
	// +optional
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}
