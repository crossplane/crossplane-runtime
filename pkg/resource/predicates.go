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

// HasClassReferenceKinds accepts ResourceClaims that reference the correct kind of resourceclass
func HasClassReferenceKinds(c client.Client, scheme runtime.ObjectCreater, k ClassKinds) PredicateFn {
	return func(obj runtime.Object) bool {
		claim, ok := obj.(Claim)
		if !ok {
			return false
		}

		pr := claim.GetPortableClassReference()
		if pr == nil {
			return false
		}

		ctx, cancel := context.WithTimeout(context.Background(), claimReconcileTimeout)
		defer cancel()

		portable := MustCreateObject(k.Portable, scheme).(PortableClass)
		p := types.NamespacedName{
			Namespace: claim.GetNamespace(),
			Name:      pr.Name,
		}
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

// NoPortableClassReference accepts ResourceClaims that do not reference a specific portable class
func NoPortableClassReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(PortableClassReferencer)
		if !ok {
			return false
		}
		return cr.GetPortableClassReference() == nil
	}
}

// NoManagedResourceReference accepts ResourceClaims that do not reference a specific Managed Resource
func NoManagedResourceReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ManagedResourceReferencer)
		if !ok {
			return false
		}
		return cr.GetResourceReference() == nil
	}
}
