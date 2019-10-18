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

	"github.com/pkg/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	attributeReferencerTagName = "attributereferencer"
	errPanicedResolving        = "paniced while resolving references: %v"
)

// Error strings
const (
	errTaggedFieldlNotImplemented = "ManagedReferenceResolver: the field has the %v tag, but has not implemented AttributeReferencer interface"
	errBuildAttribute             = "ManagedReferenceResolver: could not build the attribute"
	errAssignAttribute            = "ManagedReferenceResolver: could not assign the attribute"
)

// fieldHasTagPair is used in findAttributeReferencerFields
type fieldHasTagPair struct {
	fieldValue reflect.Value
	hasTheTag  bool
}

// NotReadyError is a custom error interface, that indicates the resource is not ready
type NotReadyError interface {
	error

	resources() []string
}

type resourceNotReadyErr struct {
	items []string
}

// NewNotReadyErr returns a new NotReadyError
func NewNotReadyErr(items []string) NotReadyError {
	return &resourceNotReadyErr{items}
}

func (r *resourceNotReadyErr) Error() string {
	return fmt.Sprintf("These resources are not ready to be referenced: %s", r.items)
}

func (r *resourceNotReadyErr) resources() []string {
	return r.items
}

// AttributeReferencer is an interface for referencing and resolving
// cross-resource attribute references. See
// https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
// for more information
type AttributeReferencer interface {

	// ValidateReady validates that the referenced managed resource type has the
	// 'Ready' condition type and it is 'True'. If the returned error is nil,
	// the resource is interpretted to be `Ready`. Otherwise, if the error type
	// is `NotReadyError`, then the reference resolution status is updated as
	// `ReferenceResolutionBlocked`, otherwise it updates the status as
	// `ReconcileError`
	ValidateReady(context.Context, Managed, client.Reader) error

	// Build retrieves referenced resource, as well as other non-managed
	// resources (like a `Provider`), and builds the referenced attribute
	Build(context.Context, Managed, client.Reader) (string, error)

	// Assign accepts a managed resource object, and assigns the given value to the
	// corresponding property
	Assign(Managed, string) error
}

// A ManagedReferenceResolver resolves the references to other managed
// resources, by looking them up in the Kubernetes API server. The references
// are the fields in the managed resource that implement AttributeReferencer
// interface and have
// `attributeReferencerTagName:"managedResourceStructTagPackageName"` tag
type ManagedReferenceResolver interface {
	ResolveReferences(context.Context, Managed) error
}

// APIManagedReferenceResolver resolves implements ManagedReferenceResolver interface
type APIManagedReferenceResolver struct {
	reader client.Reader
}

// NewReferenceResolver returns a new APIManagedReferenceResolver
func NewReferenceResolver(c client.Reader) *APIManagedReferenceResolver {
	return &APIManagedReferenceResolver{c}
}

// ResolveReferences resolves references made to other managed resources
func (r *APIManagedReferenceResolver) ResolveReferences(ctx context.Context, mg Managed) (err error) {
	// retrieve all the referencer fields from the managed resource
	referencers, err := findAttributeReferencerFields(mg, false)
	if err != nil {
		// if there is an error it should immediately panic, since this means an
		// attribute is tagged but doesn't implement AttributeReferencer
		panic(err)
	}

	// this recovers from potential panics during execution of
	// AttributeReferencer methods
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf(errPanicedResolving, r)
		}
	}()

	// make sure that all the references are ready
	for _, referencer := range referencers {
		if err := referencer.ValidateReady(ctx, mg, r.reader); err != nil {
			return err
		}
	}

	// build and assign the attributes
	for _, referencer := range referencers {
		val, err := referencer.Build(ctx, mg, r.reader)
		if err != nil {
			return errors.WithMessage(err, errBuildAttribute)
		}

		if err := referencer.Assign(mg, val); err != nil {
			return errors.WithMessage(err, errAssignAttribute)
		}
	}

	return nil
}

// findAttributeReferencerFields recursively finds all non-nil fields in a struct and its sub types
// that implement AttributeReferencer and have `attributeReferencerTagName:"managedResourceStructTagPackageName"` tag
func findAttributeReferencerFields(obj interface{}, objHasTheRightTag bool) ([]AttributeReferencer, error) {
	pairs := []fieldHasTagPair{}
	v := reflect.ValueOf(obj)

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil, nil
		}

		pairs = append(pairs, fieldHasTagPair{v.Elem(), objHasTheRightTag})

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			pairs = append(pairs, fieldHasTagPair{v.Field(i), hasAttrRefTag(reflect.TypeOf(obj).Field(i))})
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			pairs = append(pairs, fieldHasTagPair{v.Index(i), objHasTheRightTag})
		}
	}

	return inspectFields(pairs)
}

// inspectFields along with findAttributeReferencerFields it recursively
// inspects the extracted struct fields, and returns the ones that are of type
// `AttributeReferencer`.
func inspectFields(pairs []fieldHasTagPair) ([]AttributeReferencer, error) {
	result := []AttributeReferencer{}
	for _, pair := range pairs {
		if !pair.fieldValue.CanInterface() {
			if pair.hasTheTag {
				// if the field has the tag but its value cannot be converted to
				// an `Interface{}` (like a struct with private fields) it can't
				// possibly implement the method sets. returning error here
				// since, there won't be a recursive call for this value
				return nil, errors.Errorf(errTaggedFieldlNotImplemented, attributeReferencerTagName)
			}

			continue
		}

		if pair.hasTheTag {
			if ar, implements := pair.fieldValue.Interface().(AttributeReferencer); implements {
				if !pair.fieldValue.IsNil() {
					result = append(result, ar)
				}

				continue
			}

			// this is for the case where a tag is assigned to a struct, but it
			// doesn't implement the interface
			if pair.fieldValue.Kind() == reflect.Struct {
				return nil, errors.Errorf(errTaggedFieldlNotImplemented, attributeReferencerTagName)
			}
		}

		resolvers, err := findAttributeReferencerFields(pair.fieldValue.Interface(), pair.hasTheTag)
		if err != nil {
			return nil, err
		}

		result = append(result, resolvers...)
	}

	return result, nil
}

// hasAttrRefTag returns true if the given struct field has the
// AttributeReference tag
func hasAttrRefTag(field reflect.StructField) bool {
	val, ok := field.Tag.Lookup(managedResourceStructTagPackageName)
	return ok && (val == attributeReferencerTagName)
}
