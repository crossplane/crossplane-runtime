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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// A PredicateFn returns true if the supplied object should be reconciled.
type PredicateFn func(obj runtime.Object) bool

// NewPredicates returns a set of Funcs that are all satisfied by the supplied
// PredicateFn. The PredicateFn is run against the new object during updates.
func NewPredicates(fn PredicateFn) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return fn(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return fn(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return fn(e.ObjectNew) },
		GenericFunc: func(e event.GenericEvent) bool { return fn(e.Object) },
	}
}

// AnyOf accepts objects that pass any of the supplied predicate functions.
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

// HasManagedResourceReferenceKind accepts objects that reference the supplied
// managed resource kind.
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

// HasDirectClassReferenceKind accepts objects that reference the supplied
// non-portable class kind directly.
func HasDirectClassReferenceKind(k NonPortableClassKind) PredicateFn {
	return func(obj runtime.Object) bool {
		r, ok := obj.(NonPortableClassReferencer)
		if !ok {
			return false
		}

		if r.GetNonPortableClassReference() == nil {
			return false
		}

		return r.GetNonPortableClassReference().GroupVersionKind() == schema.GroupVersionKind(k)
	}
}

// HasIndirectClassReferenceKind accepts namespaced objects that reference the
// supplied non-portable class kind via the supplied portable class kind.
func HasIndirectClassReferenceKind(c client.Client, oc runtime.ObjectCreater, k ClassKinds) PredicateFn {
	return func(obj runtime.Object) bool {
		pcr, ok := obj.(PortableClassReferencer)
		if !ok {
			return false
		}

		pr := pcr.GetPortableClassReference()
		if pr == nil {
			return false
		}

		n, ok := obj.(interface{ GetNamespace() string })
		if !ok {
			return false
		}

		ctx, cancel := context.WithTimeout(context.Background(), claimReconcileTimeout)
		defer cancel()

		portable := MustCreateObject(k.Portable, oc).(PortableClass)
		p := types.NamespacedName{Namespace: n.GetNamespace(), Name: pr.Name}
		if err := c.Get(ctx, p, portable); err != nil {
			return false
		}

		cr := portable.GetNonPortableClassReference()
		if cr == nil {
			return false
		}

		gvk := cr.GroupVersionKind()

		return gvk == k.NonPortable
	}
}

// HasNoPortableClassReference accepts ResourceClaims that do not reference a
// specific portable class
func HasNoPortableClassReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(PortableClassReferencer)
		if !ok {
			return false
		}
		return cr.GetPortableClassReference() == nil
	}
}

// HasNoManagedResourceReference accepts ResourceClaims that do not reference a
// specific Managed Resource
func HasNoManagedResourceReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ManagedResourceReferencer)
		if !ok {
			return false
		}
		return cr.GetResourceReference() == nil
	}
}
