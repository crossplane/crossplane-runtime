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

package v1

import "k8s.io/apimachinery/pkg/runtime"

// PublishConnectionDetailsTo represents configuration of a connection secret.
type PublishConnectionDetailsTo struct {
	// Name is the name of the connection secret
	Name string `json:"name"`

	// Metadata is secret store specific key/value pairs to be used as metadata
	// Please note, expected keys will differ for each store type. For example,
	// it could be "labels" and "annotations" in case of "Kubernetes", but it
	// would be "tags" for "AWS Secret Manager".
	// +kubebuilder:pruning:PreserveUnknownFields
	Metadata runtime.RawExtension `json:"metadata,omitempty"`

	// SecretStoreConfigRef specifies which secret store config should be used
	// for this ConnectionSecret.
	// +optional
	// +kubebuilder:default={"name": "default"}
	SecretStoreConfigRef *Reference `json:"configRef,omitempty"`
}

// SecretStoreType represents a secret store type.
type SecretStoreType string

const (
	// SecretStoreKubernetes indicates that secret store type is
	// Kubernetes. In other words, connection secrets will be stored as K8s
	// Secrets.
	SecretStoreKubernetes SecretStoreType = "Kubernetes"

	// SecretStoreVault indicates that secret store type is Vault.
	SecretStoreVault SecretStoreType = "Vault"
)

// SecretStoreConfig represents configuration of a Secret Store.
type SecretStoreConfig struct {
	// Type configures which secret store to be used. Only the configuration
	// block for this store will be used and others will be ignored if provided.
	// Default is Kubernetes.
	// +optional
	// +kubebuilder:default=Kubernetes
	Type *SecretStoreType `json:"type,omitempty"`

	// DefaultScope used for scoping secrets for "cluster-scoped" resources.
	// If store type is "Kubernetes", this would mean the default namespace to
	// store connection secrets for cluster scoped resources.
	// In case of "Vault", this would be used as the default parent path.
	// Typically, should be set as Crossplane installation namespace.
	DefaultScope string `json:"defaultScope"`

	// Kubernetes configures a Kubernetes secret store.
	// If the "type" is "Kubernetes" but no config provided, in cluster config
	// will be used.
	// +optional
	Kubernetes *KubernetesSecretStoreConfig `json:"kubernetes,omitempty"`

	// Vault configures a Vault secret store.
	// +optional
	Vault *VaultSecretStoreConfig `json:"vault,omitempty"`
}

// KubernetesAuthConfig required to authenticate to a K8s API. It expects
// a "kubeconfig" file to be provided.
type KubernetesAuthConfig struct {
	// Source of the credentials.
	// +kubebuilder:validation:Enum=None;Secret;Environment;Filesystem
	Source CredentialsSource `json:"source"`

	// CommonCredentialSelectors provides common selectors for extracting
	// credentials.
	CommonCredentialSelectors `json:",inline"`
}

// KubernetesSecretStoreConfig represents the required configuration
// for a Kubernetes secret store.
type KubernetesSecretStoreConfig struct {
	// Credentials used to connect to the Kubernetes API.
	Auth KubernetesAuthConfig `json:"auth"`

	// TODO(turkenh): Support additional identities like
	// https://github.com/crossplane-contrib/provider-kubernetes/blob/4d722ef914e6964e80e190317daca9872ae98738/apis/v1alpha1/types.go#L34
}

// VaultAuthMethod represent a Vault authentication method.
// https://www.vaultproject.io/docs/auth
type VaultAuthMethod string

const (
	// VaultAuthKubernetes indicates that "Kubernetes Auth" will be used to
	// authenticate to Vault.
	// https://www.vaultproject.io/docs/auth/kubernetes
	VaultAuthKubernetes VaultAuthMethod = "Kubernetes"

	// VaultAuthToken indicates that "Token Auth" will be used to
	// authenticate to Vault.
	// https://www.vaultproject.io/docs/auth/token
	VaultAuthToken VaultAuthMethod = "Token"
)

// VaultAuthKubernetesConfig represents configuration for Vault Kubernetes Auth
// Method.
// https://www.vaultproject.io/docs/auth
type VaultAuthKubernetesConfig struct {
	// MountPath configures path where the Kubernetes authentication backend is
	// mounted in Vault.
	MountPath string `json:"mountPath"`

	// Role configures the Vault Role to assume.
	Role string `json:"role"`
}

// VaultAuthConfig required to authenticate to a Vault API.
type VaultAuthConfig struct {
	// Method configures which auth method will be used.
	Method VaultAuthMethod `json:"method"`
	// Kubernetes configures Kubernetes Auth for Vault.
	// +optional
	Kubernetes *VaultAuthKubernetesConfig `json:"kubernetes,omitempty"`
}

// VaultSecretStoreConfig represents the required configuration for a Vault
// secret store.
type VaultSecretStoreConfig struct {
	// Server is the url of the Vault server, e.g. "https://vault.acme.org"
	Server string `json:"server"`

	// ParentPath is the path to be prepended to all secrets.
	ParentPath string `json:"parentPath"`

	// Version of the KV Secrets engine of Vault.
	// https://www.vaultproject.io/docs/secrets/kv
	// +optional
	// +kubebuilder:default=v2
	Version *string `json:"version,omitempty"`

	// CABundle is base64 encoded string of Vaults CA certificate.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`

	// CABundleSecretRef is a reference to a K8s secret key with Vaults CA
	// certificate.
	// +optional
	CABundleSecretRef *SecretKeySelector `json:"caBundleSecretRef,omitempty"`

	// Auth configures an authentication method for Vault.
	Auth VaultAuthConfig `json:"auth"`
}
