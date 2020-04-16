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

package managed

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings
const (
	errGetReferencerStatus = "could not get referenced resource status"
	errUpdateReferencer    = "could not update resource referencer"
	errBuildAttribute      = "could not build the attribute"
	errAssignAttribute     = "could not assign the attribute"
)

// referencesAccessErr is used to indicate that one or more references can not
// be accessed
type referencesAccessErr struct {
	statuses []resource.ReferenceStatus
}

// newReferenceAccessErr returns a referencesAccessErr if any of the given
// references are not ready
func newReferenceAccessErr(statuses []resource.ReferenceStatus) error {
	for _, st := range statuses {
		if st.Status != resource.ReferenceReady {
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
//
// Deprecated: Treat reference errors as a regular reconcile error.
func IsReferencesAccessError(err error) bool {
	_, result := err.(*referencesAccessErr)
	return result
}

// An AttributeReferencerFinder returns all types within the supplied object
// that satisfy AttributeReferencer.
type AttributeReferencerFinder interface {
	FindReferencers(obj interface{}) []resource.AttributeReferencer
}

// An AttributeReferencerFinderFn satisfies AttributeReferencerFinder.
type AttributeReferencerFinderFn func(obj interface{}) []resource.AttributeReferencer

// FindReferencers finds all AttributeReferencers.
func (fn AttributeReferencerFinderFn) FindReferencers(obj interface{}) []resource.AttributeReferencer {
	return fn(obj)
}

// An APIReferenceResolver finds and resolves a resource's references,
// then updates it in the Kubernetes API.
type APIReferenceResolver struct {
	client client.Client
	finder AttributeReferencerFinder
}

// An APIReferenceResolverOption configures an
// APIReferenceResolver.
type APIReferenceResolverOption func(*APIReferenceResolver)

// WithAttributeReferencerFinder specifies an AttributeReferencerFinder used to
// find AttributeReferencers.
func WithAttributeReferencerFinder(f AttributeReferencerFinder) APIReferenceResolverOption {
	return func(r *APIReferenceResolver) {
		r.finder = f
	}
}

// NewAPIReferenceResolver returns an APIReferenceResolver. The
// resolver uses reflection to recursively finds all pointer types in a struct
// that satisfy AttributeReferencer by default. It assesses only pointers,
// structs, and slices because it is assumed that only struct fields or slice
// elements that are pointers to a struct will satisfy AttributeReferencer.
//
// Deprecated: Use NewAPISimpleReferenceResolver
func NewAPIReferenceResolver(c client.Client, o ...APIReferenceResolverOption) *APIReferenceResolver {
	r := &APIReferenceResolver{
		client: c,
		finder: AttributeReferencerFinderFn(findReferencers),
	}

	for _, rro := range o {
		rro(r)
	}

	return r
}

// ResolveReferences resolves references made to other managed resources
func (r *APIReferenceResolver) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	// Retrieve all the referencer fields from the managed resource.
	referencers := r.finder.FindReferencers(mg)

	// If there are no referencers exit early.
	if len(referencers) == 0 {
		return nil
	}

	// Make sure that all the references are ready.
	allStatuses := []resource.ReferenceStatus{}
	for _, referencer := range referencers {
		statuses, err := referencer.GetStatus(ctx, mg, r.client)
		if err != nil {
			return errors.Wrap(err, errGetReferencerStatus)
		}

		allStatuses = append(allStatuses, statuses...)
	}

	if err := newReferenceAccessErr(allStatuses); err != nil {
		return err
	}

	existing := mg.DeepCopyObject()

	// Build and assign the attributes.
	for _, referencer := range referencers {
		val, err := referencer.Build(ctx, mg, r.client)
		if err != nil {
			return errors.Wrap(err, errBuildAttribute)
		}

		if err := referencer.Assign(mg, val); err != nil {
			return errors.Wrap(err, errAssignAttribute)
		}
	}

	// Don't update if nothing changed during reference assignment.
	if cmp.Equal(existing, mg) {
		return nil
	}

	// Persist the updated managed resource.
	return errors.Wrap(r.client.Update(ctx, mg), errUpdateReferencer)
}

// findReferencers recursively finds all pointer types in a struct that satisfy
// AttributeReferencer. It assesses only pointers, structs, and slices because
// it is assumed that only struct fields or slice elements that are pointers to
// a struct will satisfy AttributeReferencer.
func findReferencers(obj interface{}) []resource.AttributeReferencer { // nolint:gocyclo
	// NOTE(negz): This function is slightly over our complexity goal, but is
	// easier to follow as a single function.

	referencers := []resource.AttributeReferencer{}

	switch v := reflect.ValueOf(obj); v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || !v.CanInterface() {
			return nil
		}
		if ar, ok := v.Interface().(resource.AttributeReferencer); ok {
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
