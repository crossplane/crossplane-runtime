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
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Error strings
const (
	errGetReferencerStatus = "could not get referenced resource status"
	errUpdateReferencer    = "could not update resource referencer"
	errBuildAttribute      = "could not build the attribute"
	errAssignAttribute     = "could not assign the attribute"
)

// ReferenceStatusType is an enum type for the possible values for a Reference Status
type ReferenceStatusType int

const (
	// ReferenceStatusUnknown is the default value
	ReferenceStatusUnknown ReferenceStatusType = iota
	// ReferenceNotFound shows that the reference is not found
	ReferenceNotFound
	// ReferenceNotReady shows that the reference is not ready
	ReferenceNotReady
	// ReferenceReady shows that the reference is ready
	ReferenceReady
)

func (t ReferenceStatusType) String() string {
	return []string{"Unknown", "NotFound", "NotReady", "Ready"}[t]
}

// ReferenceStatus has the name and status of a reference
type ReferenceStatus struct {
	Name   string
	Status ReferenceStatusType
}

func (r ReferenceStatus) String() string {
	return fmt.Sprintf("{reference:%s status:%s}", r.Name, r.Status)
}

// referencesAccessErr is used to indicate that one or more references can not
// be accessed
type referencesAccessErr struct {
	statuses []ReferenceStatus
}

// newReferenceAccessErr returns a referencesAccessErr if any of the given
// references are not ready
func newReferenceAccessErr(statuses []ReferenceStatus) error {
	for _, st := range statuses {
		if st.Status != ReferenceReady {
			return &referencesAccessErr{statuses}
		}
	}

	return nil
}

func (r *referencesAccessErr) Error() string {
	return fmt.Sprintf("%s", r.statuses)
}

// IsReferencesAccessError returns true if the given error indicates that some
// of the `AttributeReferencer` fields are referring to objects that are not
// accessible, either they are not ready or they do not yet exist
func IsReferencesAccessError(err error) bool {
	_, result := err.(*referencesAccessErr)
	return result
}

// A CanReference is a resource that can reference another resource in its
// spec in order to automatically resolve corresponding spec field values
// by inspecting the referenced resource.
type CanReference runtime.Object

// An AttributeReferencer resolves cross-resource attribute references. See
// https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
// for more information
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

// A ManagedReferenceResolver resolves the references to other managed
// resources, by looking them up in the Kubernetes API server. The references
// are the fields in the managed resource that implement AttributeReferencer
// interface and have
// `attributeReferencerTagName:"managedResourceStructTagPackageName"` tag
type ManagedReferenceResolver interface {
	ResolveReferences(context.Context, CanReference) error
}

// An AttributeReferencerFinder returns all types within the supplied object
// that satisfy AttributeReferencer.
type AttributeReferencerFinder interface {
	FindReferencers(obj interface{}) []AttributeReferencer
}

// An AttributeReferencerFinderFn satisfies AttributeReferencerFinder.
type AttributeReferencerFinderFn func(obj interface{}) []AttributeReferencer

// FindReferencers finds all AttributeReferencers.
func (fn AttributeReferencerFinderFn) FindReferencers(obj interface{}) []AttributeReferencer {
	return fn(obj)
}

// An APIManagedReferenceResolver finds and resolves a resource's references,
// then updates it in the Kubernetes API.
type APIManagedReferenceResolver struct {
	client client.Client
	finder AttributeReferencerFinder
}

// An APIManagedReferenceResolverOption configures an
// APIManagedReferenceResolver.
type APIManagedReferenceResolverOption func(*APIManagedReferenceResolver)

// WithAttributeReferencerFinder specifies an AttributeReferencerFinder used to
// find AttributeReferencers.
func WithAttributeReferencerFinder(f AttributeReferencerFinder) APIManagedReferenceResolverOption {
	return func(r *APIManagedReferenceResolver) {
		r.finder = f
	}
}

// NewAPIManagedReferenceResolver returns an APIManagedReferenceResolver. The
// resolver uses reflection to recursively finds all pointer types in a struct
// that satisfy AttributeReferencer by default. It assesses only pointers,
// structs, and slices because it is assumed that only struct fields or slice
// elements that are pointers to a struct will satisfy AttributeReferencer.
func NewAPIManagedReferenceResolver(c client.Client, o ...APIManagedReferenceResolverOption) *APIManagedReferenceResolver {
	r := &APIManagedReferenceResolver{
		client: c,
		finder: AttributeReferencerFinderFn(findReferencers),
	}

	for _, rro := range o {
		rro(r)
	}

	return r
}

// ResolveReferences resolves references made to other managed resources
func (r *APIManagedReferenceResolver) ResolveReferences(ctx context.Context, res CanReference) error {
	// Retrieve all the referencer fields from the managed resource.
	referencers := r.finder.FindReferencers(res)

	// If there are no referencers exit early.
	if len(referencers) == 0 {
		return nil
	}

	// Make sure that all the references are ready.
	allStatuses := []ReferenceStatus{}
	for _, referencer := range referencers {
		statuses, err := referencer.GetStatus(ctx, res, r.client)
		if err != nil {
			return errors.Wrap(err, errGetReferencerStatus)
		}

		allStatuses = append(allStatuses, statuses...)
	}

	if err := newReferenceAccessErr(allStatuses); err != nil {
		return err
	}

	existing := res.DeepCopyObject()

	// Build and assign the attributes.
	for _, referencer := range referencers {
		val, err := referencer.Build(ctx, res, r.client)
		if err != nil {
			return errors.Wrap(err, errBuildAttribute)
		}

		if err := referencer.Assign(res, val); err != nil {
			return errors.Wrap(err, errAssignAttribute)
		}
	}

	// Don't update if nothing changed during reference assignment.
	if cmp.Equal(existing, res) {
		return nil
	}

	// Persist the updated managed resource.
	return errors.Wrap(r.client.Update(ctx, res), errUpdateReferencer)
}

// findReferencers recursively finds all pointer types in a struct that satisfy
// AttributeReferencer. It assesses only pointers, structs, and slices because
// it is assumed that only struct fields or slice elements that are pointers to
// a struct will satisfy AttributeReferencer.
func findReferencers(obj interface{}) []AttributeReferencer { // nolint:gocyclo
	// NOTE(negz): This function is slightly over our complexity goal, but is
	// easier to follow as a single function.

	referencers := []AttributeReferencer{}

	switch v := reflect.ValueOf(obj); v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || !v.CanInterface() {
			return nil
		}
		if ar, ok := v.Interface().(AttributeReferencer); ok {
			referencers = append(referencers, ar)
		}
		if v.Elem().CanInterface() {
			referencers = append(referencers, findReferencers(v.Elem().Interface())...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			referencers = append(referencers, findReferencers(v.Field(i).Interface())...)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).CanInterface() {
				continue
			}
			referencers = append(referencers, findReferencers(v.Index(i).Interface())...)
		}
	}

	return referencers
}
