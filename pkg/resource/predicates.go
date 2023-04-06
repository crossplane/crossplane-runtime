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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// A PredicateFn returns true if the supplied object should be reconciled.
// Deprecated: This type will be removed soon. Please use
// controller-runtime's predicate.NewPredicateFuncs instead.
type PredicateFn func(obj runtime.Object) bool

// NewPredicates returns a set of Funcs that are all satisfied by the supplied
// PredicateFn. The PredicateFn is run against the new object during updates.
// Deprecated: This function will be removed soon. Please use
// controller-runtime's predicate.NewPredicateFuncs instead.
func NewPredicates(fn PredicateFn) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return fn(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return fn(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return fn(e.ObjectNew) },
		GenericFunc: func(e event.GenericEvent) bool { return fn(e.Object) },
	}
}

// AnyOf accepts objects that pass any of the supplied predicate functions.
// Deprecated: This function will be removed soon. Please use
// controller-runtime's predicate.Or instead.
func AnyOf(fn ...PredicateFn) PredicateFn {
	return func(obj runtime.Object) bool {
		for _, f := range fn {
			if f(obj) {
				return true
			}
		}
		return false
	}
}

// AllOf accepts objects that pass all of the supplied predicate functions.
// Deprecated: This function will be removed soon. Please use
// controller-runtime's predicate.And instead.
func AllOf(fn ...PredicateFn) PredicateFn {
	return func(obj runtime.Object) bool {
		for _, f := range fn {
			if !f(obj) {
				return false
			}
		}
		return true
	}
}

// HasManagedResourceReferenceKind accepts objects that reference the supplied
// managed resource kind.
// Deprecated: This function will be removed soon.
func HasManagedResourceReferenceKind(k ManagedKind) PredicateFn {
	return func(obj runtime.Object) bool {
		r, ok := obj.(ManagedResourceReferencer)
		if !ok {
			return false
		}

		if r.GetResourceReference() == nil {
			return false
		}

		return r.GetResourceReference().GroupVersionKind() == schema.GroupVersionKind(k)
	}
}

// IsManagedKind accepts objects that are of the supplied managed resource kind.
// Deprecated: This function will be removed soon.
func IsManagedKind(k ManagedKind, ot runtime.ObjectTyper) PredicateFn {
	return func(obj runtime.Object) bool {
		gvk, err := GetKind(obj, ot)
		if err != nil {
			return false
		}
		return gvk == schema.GroupVersionKind(k)
	}
}

// IsControlledByKind accepts objects that are controlled by a resource of the
// supplied kind.
// Deprecated: This function will be removed soon.
func IsControlledByKind(k schema.GroupVersionKind) PredicateFn {
	return func(obj runtime.Object) bool {
		mo, ok := obj.(metav1.Object)
		if !ok {
			return false
		}

		ref := metav1.GetControllerOf(mo)
		if ref == nil {
			return false
		}

		return ref.APIVersion == k.GroupVersion().String() && ref.Kind == k.Kind
	}
}

// IsPropagator accepts objects that request to be partially or fully propagated
// to another object of the same kind.
// Deprecated: This function will be removed soon.
func IsPropagator() PredicateFn {
	return func(obj runtime.Object) bool {
		from, ok := obj.(metav1.Object)
		if !ok {
			return false
		}

		return len(meta.AllowsPropagationTo(from)) > 0
	}
}

// IsPropagated accepts objects that consent to be partially or fully propagated
// from another object of the same kind.
// Deprecated: This function will be removed soon.
func IsPropagated() PredicateFn {
	return func(obj runtime.Object) bool {
		to, ok := obj.(metav1.Object)
		if !ok {
			return false
		}
		nn := meta.AllowsPropagationFrom(to)
		return nn.Namespace != "" && nn.Name != ""
	}
}

// IsNamed accepts objects that is named as the given name.
// Deprecated: This function will be removed soon.
func IsNamed(name string) PredicateFn {
	return func(obj runtime.Object) bool {
		mo, ok := obj.(metav1.Object)
		if !ok {
			return false
		}
		return mo.GetName() == name
	}
}

// DesiredStateChanged accepts objects that have changed their desired state, i.e.
// the state that is not managed by the controller.
// To be more specific, it accepts update events that have changes in one of the followings:
// - `metadata.annotations` (except for certain annotations)
// - `metadata.labels`
// - `spec`
func DesiredStateChanged() predicate.Predicate {
	return predicate.Or(
		AnnotationChangedPredicate{
			ignored: []string{
				// These annotations are managed by the controller and should
				// not be considered as a change in desired state. The managed
				// reconciler explicitly requests a new reconcile already after
				// updating these annotations.
				meta.AnnotationKeyExternalCreateFailed,
				meta.AnnotationKeyExternalCreatePending,
			},
		},
		predicate.LabelChangedPredicate{},
		predicate.GenerationChangedPredicate{},
	)
}

// AnnotationChangedPredicate implements a default update predicate function on
// annotation change by ignoring the given annotation keys, if any.
//
// This predicate extends controller-runtime's AnnotationChangedPredicate by
// being able to ignore certain annotations.
type AnnotationChangedPredicate struct {
	predicate.Funcs
	ignored []string
}

// Update implements default UpdateEvent filter for validating annotation change.
func (a AnnotationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		// Update event has no old object to update
		return false
	}
	if e.ObjectNew == nil {
		// Update event has no new object for update
		return false
	}

	na := e.ObjectNew.GetAnnotations()
	oa := e.ObjectOld.GetAnnotations()

	for _, k := range a.ignored {
		delete(na, k)
		delete(oa, k)
	}

	// Below is the same as controller-runtime's AnnotationChangedPredicate
	// implementation but optimized to avoid using reflect.DeepEqual.
	if len(na) != len(oa) {
		// annotation length changed
		return true
	}

	for k, v := range na {
		if oa[k] != v {
			// annotation value changed
			return true
		}
	}

	// annotations unchanged.
	return false
}
