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
	errSelectReference     = "could not select resource reference"
	errResolveReference    = "could not resolve resource reference"
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
// Deprecated: Use resource.IsReferenceError
func IsReferencesAccessError(err error) bool {
	_, result := err.(*referencesAccessErr)
	return result
}

// A Selector selects a managed resource to reference. Each Selector corresponds
// to a Referencer; for example spec.fooSelector might select a managed resource
// of kind: Foo in order to set the value of spec.fooRef.name in the referencing
// resource.
type Selector interface {
	// Select a reference to a managed resource by setting the corresponding
	// Referencer field in the referencing resource.
	Select(ctx context.Context, c client.Reader, mg resource.Managed) error
}

// An SelectorFinder returns all types within the supplied object that satisfy
// Selector.
type SelectorFinder interface {
	FindSelectors(obj interface{}) []Selector
}

// An SelectorFinderFn satisfies SelectorFinder.
type SelectorFinderFn func(obj interface{}) []Selector

// FindSelectors finds all Selectors.
func (fn SelectorFinderFn) FindSelectors(obj interface{}) []Selector {
	return fn(obj)
}

// A Referencer is a reference from one managed resource to another. Each
// Referencer corresponds to a field; for example spec.fooRef might reference
// a managed resource of kind: Foo in order to set the value of spec.foo in the
// referencing resource.
type Referencer interface {
	// Resolve a reference to a managed resource by setting the corresponding
	// field in the referencing resource.
	Resolve(ctx context.Context, c client.Reader, mg resource.Managed) error
}

// An ReferencerFinder returns all types within the supplied object that satisfy
// Referencer.
type ReferencerFinder interface {
	FindReferencers(obj interface{}) []Referencer
}

// An ReferencerFinderFn satisfies ReferencerFinder.
type ReferencerFinderFn func(obj interface{}) []Referencer

// FindReferencers finds all Referencers.
func (fn ReferencerFinderFn) FindReferencers(obj interface{}) []Referencer {
	return fn(obj)
}

// An AttributeReferencerFinder returns all types within the supplied object
// that satisfy AttributeReferencer.
//
// Deprecated: Use ReferenceFinder.
type AttributeReferencerFinder interface {
	FindReferencers(obj interface{}) []resource.AttributeReferencer
}

// An AttributeReferencerFinderFn satisfies AttributeReferencerFinder.
//
// Deprecated: Use ReferencerFinderFn.
type AttributeReferencerFinderFn func(obj interface{}) []resource.AttributeReferencer

// FindReferencers finds all AttributeReferencers.
func (fn AttributeReferencerFinderFn) FindReferencers(obj interface{}) []resource.AttributeReferencer {
	return fn(obj)
}

// An APIReferenceResolver finds and resolves a resource's references,
// then updates it in the Kubernetes API.
type APIReferenceResolver struct {
	client client.Client

	selectors   SelectorFinder
	referencers ReferencerFinder

	attributeReferencers AttributeReferencerFinder
}

// An APIReferenceResolverOption configures an
// APIReferenceResolver.
type APIReferenceResolverOption func(*APIReferenceResolver)

// WithSelectorFinder specifies an SelectorFinder used to find Selectors.
func WithSelectorFinder(f SelectorFinder) APIReferenceResolverOption {
	return func(r *APIReferenceResolver) {
		r.selectors = f
	}
}

// WithReferencerFinder specifies an ReferencerFinder used to find Referencers.
func WithReferencerFinder(f ReferencerFinder) APIReferenceResolverOption {
	return func(r *APIReferenceResolver) {
		r.referencers = f
	}
}

// WithAttributeReferencerFinder specifies an AttributeReferencerFinder used to
// find AttributeReferencers.
//
// Deprecated: Use WithReferencerFinder.
func WithAttributeReferencerFinder(f AttributeReferencerFinder) APIReferenceResolverOption {
	return func(r *APIReferenceResolver) {
		r.attributeReferencers = f
	}
}

// NewAPIReferenceResolver returns an APIReferenceResolver. The resolver finds
// all pointer types in a struct that satisfy Selector, Referencer, or
// resource.AttributeReferencer. It assesses only pointers, structs, and slices
// because it is assumed that only struct fields or slice elements that are
// pointers to a struct will satisfy these types.
func NewAPIReferenceResolver(c client.Client, o ...APIReferenceResolverOption) *APIReferenceResolver {
	r := &APIReferenceResolver{
		client:               c,
		selectors:            SelectorFinderFn(findSelectors),
		referencers:          ReferencerFinderFn(findReferencers),
		attributeReferencers: AttributeReferencerFinderFn(findAttributeReferencers),
	}

	for _, rro := range o {
		rro(r)
	}

	return r
}

// ResolveReferences selects and resolves references made to other managed
// resources.
func (r *APIReferenceResolver) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	existing := mg.DeepCopyObject()

	for _, sl := range r.selectors.FindSelectors(mg) {
		if err := sl.Select(ctx, r.client, mg); err != nil {
			return errors.Wrap(err, errSelectReference)
		}
	}

	for _, rf := range r.referencers.FindReferencers(mg) {
		if err := rf.Resolve(ctx, r.client, mg); err != nil {
			return errors.Wrap(err, errResolveReference)
		}
	}

	// TODO(negz): Remove this deprecated implementation once all providers have
	// migrated from resource.AttributeReferencer to Selector and Referencer.
	if err := r.resolveAttributeReferences(ctx, mg); err != nil {
		return errors.Wrap(err, errResolveReference)
	}

	// Don't update if nothing changed during reference resolution.
	if cmp.Equal(existing, mg) {
		return nil
	}

	return errors.Wrap(r.client.Update(ctx, mg), errUpdateManaged)
}

func (r *APIReferenceResolver) resolveAttributeReferences(ctx context.Context, res resource.CanReference) error {
	// Retrieve all the referencer fields from the managed resource.
	referencers := r.attributeReferencers.FindReferencers(res)

	// If there are no referencers exit early.
	if len(referencers) == 0 {
		return nil
	}

	// Make sure that all the references are ready.
	allStatuses := []resource.ReferenceStatus{}
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

	return nil
}

// findSelectors recursively finds all pointer types in a struct that
// satisfy Selector. It assesses only pointers, structs, and slices
// because it is assumed that only struct fields or slice elements that are
// pointers to a struct will satisfy Selector.
func findSelectors(obj interface{}) []Selector { // nolint:gocyclo
	// NOTE(negz): This function is slightly over our complexity goal, but is
	// easier to follow as a single function.

	s := []Selector{}

	switch v := reflect.ValueOf(obj); v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || !v.CanInterface() {
			return nil
		}
		if ar, ok := v.Interface().(Selector); ok {
			s = append(s, ar)
		}
		if v.Elem().CanInterface() {
			s = append(s, findSelectors(v.Elem().Interface())...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			s = append(s, findSelectors(v.Field(i).Interface())...)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).CanInterface() {
				continue
			}
			s = append(s, findSelectors(v.Index(i).Interface())...)
		}
	}

	return s
}

// findReferencers recursively finds all pointer types in a struct that
// satisfy Referencer. It assesses only pointers, structs, and slices
// because it is assumed that only struct fields or slice elements that are
// pointers to a struct will satisfy Referencer.
func findReferencers(obj interface{}) []Referencer { // nolint:gocyclo
	// NOTE(negz): This function is slightly over our complexity goal, but is
	// easier to follow as a single function.

	r := []Referencer{}

	switch v := reflect.ValueOf(obj); v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || !v.CanInterface() {
			return nil
		}
		if ar, ok := v.Interface().(Referencer); ok {
			r = append(r, ar)
		}
		if v.Elem().CanInterface() {
			r = append(r, findReferencers(v.Elem().Interface())...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			r = append(r, findReferencers(v.Field(i).Interface())...)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).CanInterface() {
				continue
			}
			r = append(r, findReferencers(v.Index(i).Interface())...)
		}
	}

	return r
}

// findAttributeReferencers recursively finds all pointer types in a struct that
// satisfy AttributeReferencer. It assesses only pointers, structs, and slices
// because it is assumed that only struct fields or slice elements that are
// pointers to a struct will satisfy AttributeReferencer.
func findAttributeReferencers(obj interface{}) []resource.AttributeReferencer { // nolint:gocyclo
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
			referencers = append(referencers, findAttributeReferencers(v.Elem().Interface())...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			referencers = append(referencers, findAttributeReferencers(v.Field(i).Interface())...)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !v.Index(i).CanInterface() {
				continue
			}
			referencers = append(referencers, findAttributeReferencers(v.Index(i).Interface())...)
		}
	}

	return referencers
}
