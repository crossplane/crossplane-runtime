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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	attributeReferencerTagName = "attributereferencer"
	errPanicedResolving        = "paniced while resolving references: %v"
)

// Error strings
const (
	errTaggedFieldlNotImplemented    = "ManagedReferenceResolver: the field has the %v tag, but has not implemented AttributeReferencer interface"
	errBuildAttribute                = "ManagedReferenceResolver: could not build the attribute"
	errAssignAttribute               = "ManagedReferenceResolver: could not assign the attribute"
	errUpdateResourceAfterAssignment = "ManagedReferenceResolver: could not update the resource after resolving references"
)

// fieldHasTagPair is used in findAttributeReferencerFields
type fieldHasTagPair struct {
	fieldValue reflect.Value
	hasTheTag  bool
}

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
	return fmt.Sprintf("%s:%s,", r.Name, r.Status)
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
	return fmt.Sprintf("Some of the referenced resources cannot be accessed. {%s}", r.statuses)
}

// IsReferencesAccessError returns true if the given error is of type referencesAccessErr
func IsReferencesAccessError(err error) bool {
	_, result := err.(*referencesAccessErr)
	return result
}

// CanReference is a type that is used as ReferenceResolver input
type CanReference interface {
	runtime.Object
	metav1.Object
}

// AttributeReferencer is an interface for referencing and resolving
// cross-resource attribute references. See
// https://github.com/crossplaneio/crossplane/blob/master/design/one-pager-cross-resource-referencing.md
// for more information
type AttributeReferencer interface {

	// GetStatus looks up the referenced objects in K8S api and returns a list
	// of ReferenceStatus
	GetStatus(context.Context, CanReference, client.Reader) ([]ReferenceStatus, error)

	// Build retrieves referenced resource, as well as other non-managed
	// resources (like a `Provider`), and builds the referenced attribute
	Build(context.Context, CanReference, client.Reader) (string, error)

	// Assign accepts a managed resource object, and assigns the given value to the
	// corresponding property
	Assign(CanReference, string) error
}

// A ManagedReferenceResolver resolves the references to other managed
// resources, by looking them up in the Kubernetes API server. The references
// are the fields in the managed resource that implement AttributeReferencer
// interface and have
// `attributeReferencerTagName:"managedResourceStructTagPackageName"` tag
type ManagedReferenceResolver interface {
	ResolveReferences(context.Context, CanReference) error
}

// APIManagedReferenceResolver resolves implements ManagedReferenceResolver interface
type APIManagedReferenceResolver struct {
	client client.Client
}

// NewReferenceResolver returns a new APIManagedReferenceResolver
func NewReferenceResolver(c client.Client) *APIManagedReferenceResolver {
	return &APIManagedReferenceResolver{c}
}

// ResolveReferences resolves references made to other managed resources
func (r *APIManagedReferenceResolver) ResolveReferences(ctx context.Context, res CanReference) (err error) {
	// retrieve all the referencer fields from the managed resource
	referencers, err := findAttributeReferencerFields(res, false)
	if err != nil {
		// if there is an error it should immediately panic, since this means an
		// attribute is tagged but doesn't implement AttributeReferencer
		panic(err)
	}

	// if there are no referencers exit early
	if len(referencers) == 0 {
		return nil
	}

	// this recovers from potential panics during execution of
	// AttributeReferencer methods
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf(errPanicedResolving, r)
		}
	}()

	// make sure that all the references are ready
	allStatuses := []ReferenceStatus{}
	for _, referencer := range referencers {
		statuses, err := referencer.GetStatus(ctx, res, r.client)
		if err != nil {
			return err
		}

		allStatuses = append(allStatuses, statuses...)
	}

	if err := newReferenceAccessErr(allStatuses); err != nil {
		return err
	}

	// build and assign the attributes
	for _, referencer := range referencers {
		val, err := referencer.Build(ctx, res, r.client)
		if err != nil {
			return errors.WithMessage(err, errBuildAttribute)
		}

		if err := referencer.Assign(res, val); err != nil {
			return errors.WithMessage(err, errAssignAttribute)
		}
	}

	// persist the updated managed resource
	return errors.WithMessage(r.client.Update(ctx, res), errUpdateResourceAfterAssignment)
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
