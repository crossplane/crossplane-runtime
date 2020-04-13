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
	"fmt"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Error strings
const (
	errGetManaged  = "cannot get managed resource"
	errListManaged = "cannot list managed resources"
)

// ReferenceStatusType is an enum type for the possible values for a Reference Status
type ReferenceStatusType int

// Reference statuses.
const (
	ReferenceStatusUnknown ReferenceStatusType = iota
	ReferenceNotFound
	ReferenceNotReady
	ReferenceReady
)

func (t ReferenceStatusType) String() string {
	return []string{"Unknown", "NotFound", "NotReady", "Ready"}[t]
}

// A ReferenceError indicates an issue was encountered while resolving a
// reference to another resource.
type ReferenceError struct {
	Name   string
	Status ReferenceStatusType
}

func (e ReferenceError) Error() string {
	// TODO(negz): Use human-friendly ReferenceStatusType strings, i.e.
	// "not ready" rather than "NotReady".
	return fmt.Sprintf("referenced resource %s is %s", e.Name, e.Status)
}

// IsReferenceError indicates an issue was encountered while resolving a
// reference to another managed resource.
func (e ReferenceError) IsReferenceError() bool {
	return true
}

// IsReferenceError is true if the supplied error indicates an issue resolving a
// reference to another managed resource.
func IsReferenceError(err error) bool {
	err = errors.Cause(err)
	_, ok := err.(interface {
		IsReferenceError() bool
	})
	return ok
}

// NewReferenceNotFoundError returns an error indicating that the reference with
// the supplied name was not found.
func NewReferenceNotFoundError(name string) ReferenceError {
	return ReferenceError{Name: name, Status: ReferenceNotFound}
}

// NewReferenceNotReadyError returns an error indicating that the reference with
// the supplied name was not ready.
func NewReferenceNotReadyError(name string) ReferenceError {
	return ReferenceError{Name: name, Status: ReferenceNotReady}
}

// ReferenceStatus has the name and status of a reference
//
// Deprecated: Use ReferenceError.
type ReferenceStatus struct {
	Name   string
	Status ReferenceStatusType
}

func (r ReferenceStatus) String() string {
	return fmt.Sprintf("{reference:%s status:%s}", r.Name, r.Status)
}

// A CanReference is a resource that can reference another resource in its
// spec in order to automatically resolve corresponding spec field values
// by inspecting the referenced resource.
//
// Deprecated: Use Managed.
type CanReference runtime.Object

// An AttributeReferencer resolves cross-resource attribute references. See
// https://github.com/crossplane/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
// for more information
//
// Deprecated: Use managed.Referencer.
type AttributeReferencer interface {
	// GetStatus retries the referenced resource, as well as other non-managed
	// resources (like a `Provider`) and reports their readiness for use as a
	// referenced resource.
	GetStatus(ctx context.Context, res CanReference, r client.Reader) ([]ReferenceStatus, error)

	// Build retrieves the referenced resource, as well as other non-managed
	// resources (like a `Provider`), and builds the referenced attribute,
	// returning it as a string value.
	Build(ctx context.Context, res CanReference, r client.Reader) (value string, err error)

	// Assign accepts a managed resource object, and assigns the given value to
	// its corresponding property.
	Assign(res CanReference, value string) error
}

// An AssignedFn should return true if a reference has already been assigned.
type AssignedFn func(from Managed) bool

// An AssignFn assigns a field in 'to' to a value extracted from 'from'.
type AssignFn func(from, to Managed)

// A ReferenceFn resolves a cross resource reference for the supplied Managed
// resource.
type ReferenceFn func(context.Context, client.Reader, Managed) error

// NewDefaultResolveFn returns a ReferenceFn suitable for resolution of most
// references from one managed resource to another.
func NewDefaultResolveFn(r v1alpha1.Reference, to Managed, resolved AssignedFn, resolve AssignFn) ReferenceFn {
	return func(ctx context.Context, c client.Reader, from Managed) error {
		// There's no need to resolve references when the 'from' (referencing)
		// managed resource is being deleted.
		if meta.WasDeleted(from) {
			return nil
		}

		// Return early if this reference has already been resolved according to
		// the supplied AssignedFn.
		if resolved(from) {
			return nil
		}

		err := c.Get(ctx, types.NamespacedName{Name: r.Name}, to)
		if kerrors.IsNotFound(err) {
			return NewReferenceNotFoundError(r.Name)
		}
		if err != nil {
			return errors.Wrap(err, errGetManaged)
		}
		if !IsConditionTrue(to.GetCondition(v1alpha1.TypeReady)) {
			return NewReferenceNotReadyError(r.Name)
		}

		// Resolve the reference by assigning a field in 'from' to a value
		// extracted from 'to' by the supplied AssignFn.
		resolve(from, to)
		return nil
	}
}

// NewDefaultSelectFn returns a ReferenceFn suitable for selection of most
// references from one managed resource to another.
func NewDefaultSelectFn(s v1alpha1.Selector, l ManagedList, selected AssignedFn, sel AssignFn) ReferenceFn {
	return func(ctx context.Context, c client.Reader, from Managed) error {
		// There's no need to select references when the 'from' (referencing)
		// managed resource is being deleted.
		if meta.WasDeleted(from) {
			return nil
		}

		// Return early if this reference has already been selected according to
		// the supplied AssignedFn.
		if selected(from) {
			return nil
		}

		if err := c.List(ctx, l, client.MatchingLabels(s.MatchLabels)); err != nil {
			return errors.Wrap(err, errListManaged)
		}

		mc := s.MatchController != nil && *s.MatchController
		for _, to := range l.GetItems() {
			// Don't select 'to' resources that don't have the same controller
			// reference as the 'from' resource is MatchController was true.
			if mc && !meta.HaveSameController(from, to) {
				continue
			}

			// Select the reference by assigning it in 'from' to a value
			// extracted from 'to' by the supplied AssignFn.
			sel(from, to)
			return nil
		}

		return nil
	}
}
