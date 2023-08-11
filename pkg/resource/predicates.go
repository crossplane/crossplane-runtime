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
	"k8s.io/apimachinery/pkg/runtime"
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

// DesiredStateChanged accepts objects that have changed their desired state, i.e.
// the state that is not managed by the controller.
// To be more specific, it accepts update events that have changes in one of the followings:
// - `metadata.annotations`
// - `metadata.labels`
// - `spec`
func DesiredStateChanged() predicate.Predicate {
	return predicate.Or(
		predicate.AnnotationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		predicate.GenerationChangedPredicate{},
	)
}
