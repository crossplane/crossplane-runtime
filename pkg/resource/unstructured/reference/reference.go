/*
Copyright 2024 The Crossplane Authors.

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

// Package reference contains references to resources.
package reference

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// A Claim is a reference to a claim.
type Claim struct {
	// APIVersion of the referenced claim.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced claim.
	Kind string `json:"kind"`

	// Name of the referenced claim.
	Name string `json:"name"`

	// Namespace of the referenced claim.
	Namespace string `json:"namespace"`
}

// A Composite is a reference to a composite.
type Composite struct {
	// APIVersion of the referenced composite.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced composite.
	Kind string `json:"kind"`

	// Name of the referenced composite.
	Name string `json:"name"`

	// Namespace of the referenced composite.
	Namespace *string `json:"namespace,omitempty"`
}

// A Composed is a reference to a resource a composite resource composes. It
// carries the Crossplane machinery an ObjectReference has no room for: the
// composition resource name the entry corresponds to, and the ordering
// constraints declared over it.
type Composed struct {
	// APIVersion of the referenced composed resource.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced composed resource.
	Kind string `json:"kind"`

	// Name of the referenced composed resource.
	Name string `json:"name,omitempty"`

	// Namespace of the referenced composed resource. Always empty for a
	// composed resource of a namespaced composite, which may only compose
	// resources in its own namespace.
	Namespace string `json:"namespace,omitempty"`

	// ResourceName is the composition resource name of the referenced
	// resource - the key a function uses for it. It is otherwise recorded only
	// in an annotation on the composed resource itself, so persisting it here
	// is what lets DependsOn be resolved without reading every composed
	// resource.
	ResourceName string `json:"resourceName,omitempty"`

	// DependsOn is the composition resource names this resource depends on.
	// Crossplane creates a resource only once everything it depends on is
	// ready, and deletes it only once nothing depends on it any more.
	DependsOn []string `json:"dependsOn,omitempty"`
}

// GroupVersionKind returns the GroupVersionKind of the composed reference.
func (c *Composed) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
}

// GroupVersionKind returns the GroupVersionKind of the claim reference.
func (c *Claim) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
}

// GroupVersionKind returns the GroupVersionKind of the composite reference.
func (c *Composite) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
}
