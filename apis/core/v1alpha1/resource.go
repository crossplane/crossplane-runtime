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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// A Reference to a named object.
type Reference struct {
	// Name of the referenced object.
	Name string `json:"name"`
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

// A Selector selects an object.
type Selector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	MatchControllerRef *bool `json:"matchControllerRef,omitempty"`
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

	// ProviderConfigReference specifies how the provider that will be used to
	// create, observe, update, and delete this managed resource should be
	// configured.
	ProviderConfigReference *Reference `json:"providerConfigRef,omitempty"`

	// ProviderReference specifies the provider that will be used to create,
	// observe, update, and delete this managed resource.
	// Deprecated: Please use ProviderConfigReference, i.e. `providerConfigRef`
	ProviderReference *Reference `json:"providerRef,omitempty"`

	// DeletionPolicy specifies what will happen to the underlying external
	// when this managed resource is deleted - either "Delete" or "Orphan" the
	// external resource. The "Delete" policy is the default when no policy is
	// specified.
	//
	// +optional
	// +kubebuilder:validation:Enum=Orphan;Delete
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

// ResourceStatus represents the observed state of a managed resource.
type ResourceStatus struct {
	ConditionedStatus `json:",inline"`
}

// A ProviderSpec defines the common way to get to the necessary objects to connect
// to the provider.
// Deprecated: Please use ProviderConfigSpec.
type ProviderSpec struct {
	// CredentialsSecretRef references a specific secret's key that contains
	// the credentials that are used to connect to the provider.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`
}

// A ProviderConfigSpec defines the common way to get to the necessary objects to connect
// to the provider.
type ProviderConfigSpec struct {
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
