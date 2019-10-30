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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

type mockReferencer struct {
	MockGetStatus func(context.Context, CanReference, client.Reader) ([]ReferenceStatus, error)
	MockBuild     func(context.Context, CanReference, client.Reader) (string, error)
	MockAssign    func(CanReference, string) error
}

func (m *mockReferencer) GetStatus(ctx context.Context, res CanReference, c client.Reader) ([]ReferenceStatus, error) {
	return m.MockGetStatus(ctx, res, c)
}

func (m *mockReferencer) Build(ctx context.Context, res CanReference, c client.Reader) (string, error) {
	return m.MockBuild(ctx, res, c)
}

func (m *mockReferencer) Assign(res CanReference, value string) error {
	return m.MockAssign(res, value)
}

func TestResolveReferences(t *testing.T) {
	errBoom := errors.New("boom")
	wantValue := "built"

	type args struct {
		ctx context.Context
		res CanReference
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		o      []APIManagedReferenceResolverOption
		args   args
		want   error
	}{
		"NoReferencersFound": {
			reason: "Should return early without error when no referencers are found.",
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return nil
				})),
			},
			args: args{
				ctx: context.Background(),
			},
			want: nil,
		},
		"GetStatusError": {
			reason: "Should return an error when a referencer.GetStatus returns an error.",
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, errBoom
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
			},
			want: errors.Wrap(errBoom, errGetReferencerStatus),
		},
		"ReferencesBlocked": {
			reason: "Should return a reference access error when a referencer.GetStatus reports unready references.",
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return []ReferenceStatus{{Status: ReferenceNotReady}}, nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
			},
			want: &referencesAccessErr{statuses: []ReferenceStatus{{Status: ReferenceNotReady}}},
		},
		"BuildValueError": {
			reason: "Should return an error when a referencer.Build returns an error.",
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ CanReference, _ client.Reader) (string, error) {
								return "", errBoom
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &MockManaged{},
			},
			want: errors.Wrap(errBoom, errBuildAttribute),
		},
		"AssignValueError": {
			reason: "Should return an error when a referencer.Assign returns an error.",
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ CanReference, _ client.Reader) (string, error) {
								return "", nil
							},
							MockAssign: func(_ CanReference, _ string) error {
								return errBoom
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &MockManaged{},
			},
			want: errors.Wrap(errBoom, errAssignAttribute),
		},
		"SuccessfulNoop": {
			reason: "Should return without error when assignment does not change CanReference.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ CanReference, _ client.Reader) (string, error) {
								return wantValue, nil
							},
							MockAssign: func(res CanReference, gotValue string) error {
								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &MockManaged{},
			},
			want: nil,
		},
		"SuccessfulUpdate": {
			reason: "Should return without error when a value is successfully built and assigned.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ CanReference, _ client.Reader) (string, error) {
								return wantValue, nil
							},
							MockAssign: func(res CanReference, gotValue string) error {
								if diff := cmp.Diff(wantValue, gotValue); diff != "" {
									reason := "referencer.Assign should be called with the value returned by referencer.Build."
									t.Errorf("\nReason: %s\nreferencer.Assign(...):\n%s", reason, diff)
								}

								// Simulate assignment by changing something about the resource.
								res.(*MockManaged).SetAnnotations(map[string]string{"assigned": "true"})
								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &MockManaged{},
			},
			want: nil,
		},
		"UpdateCanReferenceError": {
			reason: "Should return an error when CanReference cannot be updated.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			o: []APIManagedReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []AttributeReferencer {
					return []AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ CanReference, _ client.Reader) ([]ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ CanReference, _ client.Reader) (string, error) {
								return "", nil
							},
							MockAssign: func(res CanReference, _ string) error {
								// Simulate assignment by changing something about the resource.
								res.(*MockManaged).SetAnnotations(map[string]string{"assigned": "true"})

								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &MockManaged{},
			},
			want: errors.Wrap(errBoom, errUpdateReferencer),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIManagedReferenceResolver(tc.c, tc.o...)
			got := r.ResolveReferences(tc.args.ctx, tc.args.res)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\r.ResolveReferences(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFindReferencers(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    interface{}
		want   []AttributeReferencer
	}{
		"ObjIsNil": {
			reason: "The root object is nil, and therefore should not satisfy AttributeReferencer.",
		},
		"ObjIsNilAttributeReferencer": {
			reason: "The root object satisfies AttributeReferencer, but is nil and thus presumed unsafe to call.",
			obj:    (*mockReferencer)(nil),
		},
		"ObjIsAttributeReferencer": {
			reason: "The root object should satisfy AttributeReferencer.",
			obj:    &mockReferencer{},
			want:   []AttributeReferencer{&mockReferencer{}},
		},
		"FieldIsAttributeReferencer": {
			reason: "The root is a struct with a field object should satisfy AttributeReferencer.",
			obj: struct {
				Referencer *mockReferencer
			}{
				Referencer: &mockReferencer{},
			},
			want: []AttributeReferencer{&mockReferencer{}},
		},
		"FieldInPointerToStructIsAttributeReferencer": {
			reason: "The root object is a pointer to struct with a field that should satisfy AttributeReferencer.",
			obj: func() interface{} {
				obj := struct {
					Referencer *mockReferencer
				}{
					Referencer: &mockReferencer{},
				}
				return &obj
			}(),
			want: []AttributeReferencer{&mockReferencer{}},
		},
		"FieldIsNotAttributeReferencer": {
			reason: "The root object is a struct whose only field should not satisfy AttributeReferencer.",
			obj: struct {
				Unused string
			}{
				Unused: "notareferencer",
			},
			want: []AttributeReferencer{},
		},
		"FieldIsNotExported": {
			reason: "The root object is a struct whose only field satisfies AttributeReferencer, but is not exported and should thus fail CanInterface.",
			obj: struct {
				referencer *mockReferencer
			}{
				referencer: &mockReferencer{},
			},
			want: []AttributeReferencer{},
		},
		"ElementIsAttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should satisfy AttributeReferencer.",
			obj: struct {
				Referencers []AttributeReferencer
			}{
				Referencers: []AttributeReferencer{&mockReferencer{}},
			},
			want: []AttributeReferencer{&mockReferencer{}},
		},
		"ElementIsNotAttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should not satisfy AttributeReferencer.",
			obj: struct {
				Unused []string
			}{
				Unused: []string{"notareferencer"},
			},
			want: []AttributeReferencer{},
		},
		"MockManagedIsNotAttributeReferencer": {
			reason: "MockManaged is relatively complex, but should not break findReferencers",
			obj:    &MockManaged{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := findReferencers(tc.obj)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\nReason: %s\nfindReferencers(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
