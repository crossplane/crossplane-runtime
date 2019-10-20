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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

type ItemAReferencer struct {
	*corev1.LocalObjectReference

	getStatusFn     func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error)
	getStatusCalled bool
	buildFn         func(context.Context, Managed, client.Reader) (string, error)
	buildCalled     bool
	assignFn        func(Managed, string) error
	assignCalled    bool
	assignParam     string
}

func (r *ItemAReferencer) GetStatus(ctx context.Context, mg Managed, reader client.Reader) ([]ReferenceStatus, error) {
	r.getStatusCalled = true
	return r.getStatusFn(ctx, mg, reader)
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

func (r *ItemBReferencer) GetStatus(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) {
	return nil, nil
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

	validGetStatusFn := func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) { return nil, nil }
	validBuildFn := func(context.Context, Managed, client.Reader) (string, error) { return "fakeValue", nil }
	validAssignFn := func(Managed, string) error { return nil }

	errBoom := errors.New("boom")

	type managed struct {
		MockManaged
		ItemARef *ItemAReferencer `resource:"attributereferencer"`
	}

	type args struct {
		field            *ItemAReferencer
		clientUpdaterErr error
	}

	type want struct {
		getStatusCalled bool
		buildCalled     bool
		assignCalled    bool
		assignParam     string
		err             error
	}

	for name, tc := range map[string]struct {
		args args
		want want
	}{
		"ValidAttribute_ReturnsNil": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: validGetStatusFn,
					buildFn:     validBuildFn,
					assignFn:    validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				buildCalled:     true,
				assignCalled:    true,
				assignParam:     "fakeValue",
			},
		},
		"ValidAttribute_GetStatusError_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) {
						return nil, errBoom
					},
					buildFn:  validBuildFn,
					assignFn: validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				err:             errBoom,
			},
		},
		"ValidAttribute_GetStatusReturnsNotReadyStatus_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) {
						return []ReferenceStatus{{"cool-res", ReferenceNotReady}}, nil
					},
					buildFn:  validBuildFn,
					assignFn: validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				err:             &referencesAccessErr{[]ReferenceStatus{{"cool-res", ReferenceNotReady}}},
			},
		},
		"ValidAttribute_GetStatusReturnsMixedReadyStatus_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) {
						return []ReferenceStatus{
							{"cool1-res", ReferenceNotFound},
							{"cool2-res", ReferenceReady},
						}, nil
					},
					buildFn:  validBuildFn,
					assignFn: validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				err: &referencesAccessErr{[]ReferenceStatus{
					{"cool1-res", ReferenceNotFound},
					{"cool2-res", ReferenceReady},
				}},
			},
		},
		"ValidAttribute_GetStatusReturnsReadyStatus_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: func(context.Context, Managed, client.Reader) ([]ReferenceStatus, error) {
						return []ReferenceStatus{{"cool-res", ReferenceReady}}, nil
					},
					buildFn:  validBuildFn,
					assignFn: validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				buildCalled:     true,
				assignCalled:    true,
				assignParam:     "fakeValue",
			},
		},
		"ValidAttribute_BuildError_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: validGetStatusFn,
					buildFn:     func(context.Context, Managed, client.Reader) (string, error) { return "", errBoom },
					assignFn:    validAssignFn,
				},
			},
			want: want{
				getStatusCalled: true,
				buildCalled:     true,
				err:             errors.WithMessage(errBoom, errBuildAttribute),
			},
		},
		"ValidAttribute_AssignError_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: validGetStatusFn,
					buildFn:     validBuildFn,
					assignFn:    func(Managed, string) error { return errBoom },
				},
			},
			want: want{
				getStatusCalled: true,
				buildCalled:     true,
				assignCalled:    true,
				assignParam:     "fakeValue",
				err:             errors.WithMessage(errBoom, errAssignAttribute),
			},
		},
		"ValidAttribute_UpdateResourceError_ReturnsErr": {
			args: args{
				field: &ItemAReferencer{
					getStatusFn: validGetStatusFn,
					buildFn:     validBuildFn,
					assignFn:    validAssignFn,
				},
				clientUpdaterErr: errBoom,
			},
			want: want{
				getStatusCalled: true,
				buildCalled:     true,
				assignCalled:    true,
				assignParam:     "fakeValue",
				err:             errors.WithMessage(errBoom, errUpdateResourceAfterAssignment),
			},
		},
		"ValidAttribute_PanicHappens_Recovers": {
			args: args{
				field: &ItemAReferencer{
					// this should cause a panic, since functions are all nil
				},
			},
			want: want{
				getStatusCalled: true,
				err:             errors.Errorf(errPanicedResolving, "runtime error: invalid memory address or nil pointer dereference"),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {

			c := mockClient{updaterErr: tc.args.clientUpdaterErr}
			rr := NewReferenceResolver(&c)
			ctx := context.Background()

			res := managed{
				ItemARef: tc.args.field,
			}

			err := rr.ResolveReferences(ctx, &res)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveReferences(...): -want error, +got error:\n%s", diff)
			}

			gotCalls := []bool{tc.args.field.getStatusCalled, tc.args.field.buildCalled, tc.args.field.assignCalled}
			wantCalls := []bool{tc.want.getStatusCalled, tc.want.buildCalled, tc.want.assignCalled}

			if diff := cmp.Diff(wantCalls, gotCalls); diff != "" {
				t.Errorf("ResolveReferences(...) => []{getStatusCalled, buildCalled, assignCalled}, : -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.assignParam, tc.args.field.assignParam); diff != "" {
				t.Errorf("ResolveReferences(...) => []{getStatusCalled, buildCalled, assignCalled}, : -want, +got:\n%s", diff)
			}
		})
	}
}

type mockClient struct {
	updaterErr error
	client.Client
}

func (c *mockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return c.updaterErr
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

		NewReferenceResolver(struct{ client.Client }{}).
			ResolveReferences(context.Background(), &res)
	}()

	if diff := cmp.Diff(paniced, true); diff != "" {
		t.Errorf("ResolveReferences(...) should panic for invalid attributereferencer: -want , +got :\n%s", diff)
	}
}

func Test_ResolveReferences_NoReferencersFound_ExitsEarly(t *testing.T) {
	type mockStruct struct {
		Name string
	}

	res := struct {
		MockManaged
	}{}

	var wantErr error = nil
	gotErr := NewReferenceResolver(struct{ client.Client }{}).
		ResolveReferences(context.Background(), &res)

	if diff := cmp.Diff(wantErr, gotErr, test.EquateErrors()); diff != "" {
		t.Errorf("ResolveReferences(...) with no referencers: -want error, +got error:\n%s", diff)
	}
}
