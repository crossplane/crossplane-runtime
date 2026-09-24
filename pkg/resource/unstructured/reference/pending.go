/*
Copyright 2026 The Crossplane Authors.

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

package reference

// An Operation is a change to a composed resource that ordering can hold back.
type Operation string

const (
	// OperationCreate is a composed resource that does not exist yet.
	OperationCreate Operation = "Create"

	// OperationDelete is a composed resource that exists and is no longer
	// desired, but that something still depends on.
	OperationDelete Operation = "Delete"
)

// A DependencyType says what kind of thing a dependency points at.
type DependencyType string

const (
	// DependencyTypeComposedResource is another resource the same composite
	// composes, named by its composition resource name.
	DependencyTypeComposedResource DependencyType = "ComposedResource"

	// DependencyTypeRequiredResource is a resource the pipeline required
	// rather than composed. Crossplane never deletes a resource it didn't
	// compose, so these order creation only.
	DependencyTypeRequiredResource DependencyType = "RequiredResource"
)

// A Dependency is one ordering constraint declared over a composed resource.
type Dependency struct {
	// Type of thing this depends on. Defaults to ComposedResource.
	// +optional
	Type DependencyType `json:"type,omitempty"`

	// Name is the composition resource name depended on. Set when Type is
	// ComposedResource.
	// +optional
	Name string `json:"name,omitempty"`

	// Requirement identifies a resource the pipeline required. Set when Type
	// is RequiredResource.
	// +optional
	Requirement *RequirementDependency `json:"requirement,omitempty"`
}

// A RequirementDependency identifies a resource the pipeline required, or one
// resource within the set a requirement matched.
type RequirementDependency struct {
	// Name of the requirement - the key into the pipeline's requirements.
	Name string `json:"name"`

	// ResourceName optionally narrows the dependency to a single resource
	// within the set the requirement matched. If unset, every match must be
	// ready.
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// Namespace of ResourceName, for a namespaced resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// A Pending is a composed resource with a change ordering has not allowed yet,
// and the reason it hasn't.
//
// It is not a reference, despite living beside them. A resource held back from
// being created has no object to point at - and deliberately no entry in
// resourceRefs either, because a reference to something that doesn't exist
// reads as an error rather than as waiting. This is the only place it appears.
type Pending struct {
	// APIVersion of the composed resource.
	APIVersion string `json:"apiVersion"`

	// Kind of the composed resource.
	Kind string `json:"kind"`

	// Name of the composed resource. Set only when Operation is Delete,
	// because only then does the object exist.
	//
	// Crossplane assigns composed resource names before it applies them, so a
	// pending creation could carry one - but a name for an object that does
	// not exist is what resourceRefs avoids, and everything that reads a name
	// reads it as a pointer.
	// +optional
	Name string `json:"name,omitempty"`

	// Namespace of the composed resource, where it has one.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// ResourceName is the composition resource name - the key a function uses
	// for this resource, and what the edges in DependsOn refer to.
	ResourceName string `json:"resourceName"`

	// Operation is the change being held back.
	Operation Operation `json:"operation"`

	// DependsOn is what this resource is waiting for. Set for a pending
	// creation; a pending deletion carries its edges on its entry in
	// resourceRefs, and Reason names what still depends on it.
	// +optional
	DependsOn []Dependency `json:"dependsOn,omitempty"`

	// Reason says why the operation is held back, in a form meant to be read
	// by a person.
	//
	// It must be stable while the situation is: no elapsed times, no
	// timestamps, no counters. Crossplane skips the status write when status
	// hasn't changed, and a reason that moves on its own turns every
	// reconcile into a write - and every write into a watch event on the
	// composite, which is also a token from the watch circuit breaker. How
	// long something has been waiting belongs in a condition's
	// lastTransitionTime, which records it for free and stays put.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Deadlocked is true when waiting cannot resolve this - a cycle, or a
	// dependency on something that will never be created. Someone has to act.
	// +optional
	Deadlocked bool `json:"deadlocked,omitempty"`
}
