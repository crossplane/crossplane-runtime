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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

type ItemAReferencer struct {
	*corev1.LocalObjectReference

	validateFn     func(context.Context, Managed, client.Reader) error
	validateCalled bool
	buildFn        func(context.Context, Managed, client.Reader) (string, error)
	buildCalled    bool
	assignFn       func(Managed, string) error
	assignCalled   bool
	assignParam    string
}

func (r *ItemAReferencer) ValidateReady(ctx context.Context, mg Managed, reader client.Reader) error {
	r.validateCalled = true
	return r.validateFn(ctx, mg, reader)
}

func (r *ItemAReferencer) Build(ctx context.Context, mg Managed, reader client.Reader) (string, error) {
	r.buildCalled = true
	return r.buildFn(ctx, mg, reader)
}

func (r *ItemAReferencer) Assign(mg Managed, val string) error {
	r.assignCalled = true
	r.assignParam = val
	return r.assignFn(mg, val)
}

type ItemBReferencer struct {
	*corev1.LocalObjectReference
}

func (r *ItemBReferencer) ValidateReady(context.Context, Managed, client.Reader) error {
	return nil
}

func (r *ItemBReferencer) Build(context.Context, Managed, client.Reader) (string, error) {
	return "", nil
}

func (r *ItemBReferencer) Assign(Managed, string) error {
	return nil
}

func Test_findAttributeReferencerFields(t *testing.T) {
	// some structs that are used in this test
	type mockStruct struct {
		Name string
	}

	type MockInnerStruct struct {
		ItemARef *ItemAReferencer `resource:"attributereferencer"`
	}

	type args struct {
		o interface{}
	}

	type want struct {
		arrLen int
		err    error
	}

	// test cases
	cases := map[string]struct {
		args args
		want want
	}{
		"ValidResourceWithSingleReferencer_AsObject_ShouldReturnExpected": {
			args: args{
				o: struct {
					NonReferencerField mockStruct
					ItemARef           *ItemAReferencer `resource:"attributereferencer"`
				}{
					NonReferencerField: mockStruct{},
					ItemARef:           &ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"item-name"}},
				},
			},
			want: want{
				arrLen: 1,
				err:    nil,
			},
		},
		"ValidResourceWithSingleReferencer_AsPointer_ShouldReturnExpected": {
			args: args{
				o: &struct {
					NonReferencerField mockStruct
					ItemARef           *ItemAReferencer `resource:"attributereferencer"`
				}{
					NonReferencerField: mockStruct{},
					ItemARef:           &ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"item-name"}},
				},
			},
			want: want{
				arrLen: 1,
			},
		},
		"ValidResourceWithSingleReferencer_NilReferencer_ShouldReturnEmpty": {
			args: args{
				o: &struct {
					NonReferencerField mockStruct
					ItemARef           *ItemAReferencer `resource:"attributereferencer"`
				}{
					NonReferencerField: mockStruct{},
				},
			},
			want: want{},
		},
		"NilResource_ShouldReturnEmpty": {
			args: args{
				o: nil,
			},
			want: want{},
		},
		"ValidResourceWithMultipleReferencers_AllReferencersArePopulated_ShouldReturnExpected": {
			args: args{
				o: &struct {
					ItemARef *ItemAReferencer `resource:"attributereferencer"`
					ItemBRef *ItemBReferencer `resource:"attributereferencer"`
					AStruct  *MockInnerStruct
				}{
					ItemARef: &ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"itemA-name"}},
					ItemBRef: &ItemBReferencer{LocalObjectReference: &corev1.LocalObjectReference{"itemB-name"}},
					AStruct: &MockInnerStruct{
						&ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"itemA-name"}},
					},
				},
			},
			want: want{
				arrLen: 3,
			},
		},
		"ValidResourceWithMultipleReferencers_ReferencersArePartiallyPopulated_ShouldReturnExpected": {
			args: args{
				o: &struct {
					ItemARef *ItemAReferencer `resource:"attributereferencer"`
					ItemBRef *ItemBReferencer `resource:"attributereferencer"`
					AStruct  *MockInnerStruct
				}{
					ItemBRef: &ItemBReferencer{&corev1.LocalObjectReference{"itemB-name"}},
					AStruct: &MockInnerStruct{
						&ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"itemA-name"}},
					},
				},
			},
			want: want{
				arrLen: 2,
			},
		},
		"ValidResourceWithListOfReferencers_ListIsPopulated_ShouldReturnExpected": {
			args: args{
				o: &struct {
					ItemsRef []*ItemAReferencer `resource:"attributereferencer"`
				}{
					ItemsRef: []*ItemAReferencer{
						{LocalObjectReference: &corev1.LocalObjectReference{"itemA1-name"}},
						{LocalObjectReference: &corev1.LocalObjectReference{"itemA2-name"}},
					},
				},
			},
			want: want{
				arrLen: 2,
			},
		},
		"ValidResourceWithListOfReferencers_ListIsEmpty_ShouldReturnEmpty": {
			args: args{
				o: &struct {
					ItemsRef []*ItemAReferencer `resource:"attributereferencer"`
				}{
					ItemsRef: []*ItemAReferencer{},
				},
			},
			want: want{},
		},
		"ResourceWithNotImplementingTaggedReferencers_ShouldReturnError": {
			args: args{
				o: struct {
					// InvalidRef is tagged, but mockStruct doesn't implement
					// the required interface
					InvalidRef *mockStruct `resource:"attributereferencer"`
				}{
					InvalidRef: &mockStruct{"something"},
				},
			},
			want: want{
				err: errors.Errorf(errTaggedFieldlNotImplemented, attributeReferencerTagName),
			},
		},
		"ResourceWithNotInterfaceableTaggedReferencers_ShouldReturnError": {
			args: args{
				o: struct {
					// since nonReferencerField is not exported, its value is
					// not interfaceable
					nonReferencerField mockStruct `resource:"attributereferencer"`
				}{
					nonReferencerField: mockStruct{"something else"},
				},
			},
			want: want{
				err: errors.Errorf(errTaggedFieldlNotImplemented, attributeReferencerTagName),
			},
		},
		"ResourceWithUntaggedReferencers_ShouldReturnEmpty": {
			args: args{
				o: struct {
					// even though UntaggedRef has implemented the interface,
					// but its not tagged
					UntaggedRef *ItemAReferencer
				}{
					UntaggedRef: &ItemAReferencer{LocalObjectReference: &corev1.LocalObjectReference{"itemA-name"}},
				},
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := findAttributeReferencerFields(tc.args.o, false)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("findAttributeReferencerFields(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.arrLen, len(got)); diff != "" {
				t.Errorf("findAttributeReferencerFields(...): -want len, +got len:\n%s", diff)
			}
		})
	}
}

func Test_ResolveReferences(t *testing.T) {

	validValidateFn := func(context.Context, Managed, client.Reader) error { return nil }
	validBuildFn := func(context.Context, Managed, client.Reader) (string, error) { return "fakeValue", nil }
	validAssignFn := func(Managed, string) error { return nil }

	errBoom := errors.New("boom")

	type managed struct {
		MockManaged
		ItemARef *ItemAReferencer `resource:"attributereferencer"`
	}

	type mockReader struct{ client.Reader }
	rr := NewReferenceResolver(mockReader{})
	ctx := context.Background()

	type args struct {
		field *ItemAReferencer
	}

	type want struct {
		validateCalled bool
		buildCalled    bool
		assignCalled   bool
		assignParam    string
		err            error
	}

	for name, tc := range map[string]struct {
		args args
		want want
	}{
		"ValidAttribute_ReturnsNil": {
			args: args{
				&ItemAReferencer{
					validateFn: validValidateFn,
					buildFn:    validBuildFn,
					assignFn:   validAssignFn,
				},
			},
			want: want{
				validateCalled: true,
				buildCalled:    true,
				assignCalled:   true,
				assignParam:    "fakeValue",
			},
		},
		"ValidAttribute_ValidateError_ReturnsErr": {
			args: args{
				&ItemAReferencer{
					validateFn: func(context.Context, Managed, client.Reader) error {
						return errBoom
					},
					buildFn:  validBuildFn,
					assignFn: validAssignFn,
				},
			},
			want: want{
				validateCalled: true,
				err:            errBoom,
			},
		},
		"ValidAttribute_BuildError_ReturnsErr": {
			args: args{
				&ItemAReferencer{
					validateFn: validValidateFn,
					buildFn:    func(context.Context, Managed, client.Reader) (string, error) { return "", errBoom },
					assignFn:   validAssignFn,
				},
			},
			want: want{
				validateCalled: true,
				buildCalled:    true,
				err:            errors.WithMessage(errBoom, errBuildAttribute),
			},
		},
		"ValidAttribute_AssignError_ReturnsErr": {
			args: args{
				&ItemAReferencer{
					validateFn: validValidateFn,
					buildFn:    validBuildFn,
					assignFn:   func(Managed, string) error { return errBoom },
				},
			},
			want: want{
				validateCalled: true,
				buildCalled:    true,
				assignCalled:   true,
				assignParam:    "fakeValue",
				err:            errors.WithMessage(errBoom, errAssignAttribute),
			},
		},
		"ValidAttribute_PanicHappens_Recovers": {
			args: args{
				&ItemAReferencer{
					// this should cause a panic, since functions are all nil
				},
			},
			want: want{
				validateCalled: true,
				err:            errors.Errorf(errPanicedResolving, "runtime error: invalid memory address or nil pointer dereference"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {

			res := managed{
				ItemARef: tc.args.field,
			}

			err := rr.ResolveReferences(ctx, &res)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveReferences(...): -want error, +got error:\n%s", diff)
			}

			gotCalls := []bool{tc.args.field.validateCalled, tc.args.field.buildCalled, tc.args.field.assignCalled}
			wantCalls := []bool{tc.want.validateCalled, tc.want.buildCalled, tc.want.assignCalled}

			if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
				t.Errorf("ResolveReferences(...) => []{validateCalled, buildCalled, assignCalled}, : -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.assignParam, tc.args.field.assignParam); diff != "" {
				t.Errorf("ResolveReferences(...) => []{validateCalled, buildCalled, assignCalled}, : -want, +got:\n%s", diff)
			}
		})
	}
}

func Test_ResolveReferences_AttributeNotImplemented_Panics(t *testing.T) {
	type mockStruct struct {
		Name string
	}

	res := struct {
		MockManaged
		ItemRef mockStruct `resource:"attributereferencer"`
	}{}

	paniced := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				paniced = true
			}
		}()

		NewReferenceResolver(struct{ client.Reader }{}).
			ResolveReferences(context.Background(), &res)
	}()

	if diff := cmp.Diff(paniced, true); diff != "" {
		t.Errorf("ResolveReferences(...) should panic for invalid attributereferencer: -want , +got :\n%s", diff)
	}
}
