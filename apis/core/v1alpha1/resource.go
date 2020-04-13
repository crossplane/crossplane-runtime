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

	// NOTE(negz): Importing this as metav1 appears to break controller-gen's
	// deepcopy generation logic. It generates a deepcopy file that omits this
	// import and thus does not compile. Importing as v1 fixes this. ¯\_(ツ)_/¯
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// The annotation used to make a resource class the default.
const (
	AnnotationDefaultClassKey   = "resourceclass.crossplane.io/is-default-class"
	AnnotationDefaultClassValue = "true"
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
	// ResourceCredentialsSecretTokenKey is the key inside a connection secret for the bearer token value
	ResourceCredentialsSecretTokenKey = "token"
	// ResourceCredentialsSecretKubeconfigKey is the key inside a connection secret for the raw kubeconfig yaml
	ResourceCredentialsSecretKubeconfigKey = "kubeconfig"
)

// NOTE(negz): The below secret references differ from ObjectReference and
// LocalObjectReference in that they include only the fields Crossplane needs to
// reference a secret, and make those fields required. This reduces ambiguity in
// the API for resource authors.

// A LocalSecretReference is a reference to a secret in the same namespace as
// the referencer.
type LocalSecretReference struct {
	// Name of the secret.
	Name string `json:"name"`
}

// A SecretReference is a reference to a secret in an arbitrary namespace.
type SecretReference struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace"`
}

// A SecretKeySelector is a reference to a secret key in an arbitrary namespace.
type SecretKeySelector struct {
	SecretReference `json:",inline"`

	// The key to select.
	Key string `json:"key"`
}

// A TypedReference refers to an object by Name, Kind, and APIVersion. It is
// commonly used to reference cluster-scoped objects or objects where the
// namespace is already known.
type TypedReference struct {
	// APIVersion of the referenced object.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced object.
	Kind string `json:"kind"`

	// Name of the referenced object.
	Name string `json:"name"`

	// UID of the referenced object.
	// +optional
	UID types.UID `json:"uid,omitempty"`
}

// SetGroupVersionKind sets the Kind and APIVersion of a TypedReference.
func (obj *TypedReference) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}

// GroupVersionKind gets the GroupVersionKind of a TypedReference.
func (obj *TypedReference) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

// GetObjectKind get the ObjectKind of a TypedReference.
func (obj *TypedReference) GetObjectKind() schema.ObjectKind { return obj }

// A ResourceClaimSpec defines the desired state of a resource claim.
type ResourceClaimSpec struct {
	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this resource claim, to which any connection details
	// for this resource claim should be written. Connection details frequently
	// include the endpoint, username, and password required to connect to the
	// managed resource bound to this resource claim.
	// +optional
	WriteConnectionSecretToReference *LocalSecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// TODO(negz): Make the below references immutable once set? Doing so means
	// we don't have to track what provisioner was used to create a resource.

	// A ClassSelector specifies labels that will be used to select a resource
	// class for this claim. If multiple classes match the labels one will be
	// chosen at random.
	// +optional
	ClassSelector *v1.LabelSelector `json:"classSelector,omitempty"`

	// A ClassReference specifies a resource class that will be used to
	// dynamically provision a managed resource when the resource claim is
	// created.
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
	// WriteConnectionSecretToReference specifies the namespace and name of a
	// Secret to which any connection details for this managed resource should
	// be written. Connection details frequently include the endpoint, username,
	// and password required to connect to the managed resource.
	// +optional
	WriteConnectionSecretToReference *SecretReference `json:"writeConnectionSecretToRef,omitempty"`

	// ClaimReference specifies the resource claim to which this managed
	// resource will be bound. ClaimReference is set automatically during
	// dynamic provisioning. Crossplane does not currently support setting this
	// field manually, per https://github.com/crossplane/crossplane-runtime/issues/19
	// +optional
	ClaimReference *corev1.ObjectReference `json:"claimRef,omitempty"`

	// ClassReference specifies the resource class that was used to dynamically
	// provision this managed resource, if any. Crossplane does not currently
	// support setting this field manually, per
	// https://github.com/crossplane/crossplane-runtime/issues/20
	// +optional
	ClassReference *corev1.ObjectReference `json:"classRef,omitempty"`

	// ProviderReference specifies the provider that will be used to create,
	// observe, update, and delete this managed resource.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// ReclaimPolicy specifies what will happen to this managed resource when
	// its resource claim is deleted, and what will happen to the underlying
	// external resource when the managed resource is deleted. The "Delete"
	// policy causes the managed resource to be deleted when its bound resource
	// claim is deleted, and in turn causes the external resource to be deleted
	// when its managed resource is deleted. The "Retain" policy causes the
	// managed resource to be retained, in binding phase "Released", when its
	// resource claim is deleted, and in turn causes the external resource to be
	// retained when its managed resource is deleted. The "Retain" policy is
	// used when no policy is specified.
	// +optional
	// +kubebuilder:validation:Enum=Retain;Delete
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
	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of managed resources dynamically provisioned using
	// this claim will be created.
	WriteConnectionSecretsToNamespace string `json:"writeConnectionSecretsToNamespace"`

	// ProviderReference specifies the provider that will be used to create,
	// observe, update, and delete managed resources that are dynamically
	// provisioned using this resource class.
	ProviderReference *corev1.ObjectReference `json:"providerRef"`

	// ReclaimPolicy specifies what will happen to managed resources dynamically
	// provisioned using this class when their resource claims are deleted, and
	// what will happen to their underlying external resource when they are
	// deleted. The "Delete" policy causes the managed resource to be deleted
	// when its bound resource claim is deleted, and in turn causes the external
	// resource to be deleted when its managed resource is deleted. The "Retain"
	// policy causes the managed resource to be retained, in binding phase
	// "Released", when its resource claim is deleted, and in turn causes the
	// external resource to be retained when its managed resource is deleted.
	// The "Retain" policy is used when no policy is specified, however the
	// "Delete" policy is set at dynamic provisioning time if no policy is set.
	// +optional
	// +kubebuilder:validation:Enum=Retain;Delete
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// A ProviderSpec defines the common way to get to the necessary objects to connect
// to the provider.
type ProviderSpec struct {
	// CredentialsSecretRef references a specific secret's key that contains
	// the credentials that are used to connect to the provider.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`
}

// A TargetSpec defines the common fields of objects used for exposing
// infrastructure to workloads that can be scheduled to.
type TargetSpec struct {
	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this target, to which any connection details for this
	// target should be written or already exist. Connection secrets referenced
	// by a target should contain information for connecting to a resource that
	// allows for scheduling of workloads.
	// +optional
	WriteConnectionSecretToReference *LocalSecretReference `json:"connectionSecretRef,omitempty"`

	// A ResourceReference specifies an existing managed resource, in any
	// namespace, which this target should attempt to propagate a connection
	// secret from.
	// +optional
	ResourceReference *corev1.ObjectReference `json:"clusterRef,omitempty"`
}

// A TargetStatus defines the observed status a target.
type TargetStatus struct {
	ConditionedStatus `json:",inline"`
}

// A Reference from one managed resource to another.
type Reference struct {
	// Name of the referenced managed resource.
	Name string `json:"name"`
}

// A Selector for a Reference from one managed resource to another.
type Selector struct {
	// MatchController ensures that only managed resources with the same
	// controller reference as the selecting resource will be selected.
	MatchController *bool `json:"matchController,omitempty"`

	// MatchLabels ensures that only managed resources with matching labels will
	// be selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
