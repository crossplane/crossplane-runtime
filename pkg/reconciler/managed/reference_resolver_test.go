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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type mockReferencer struct {
	MockGetStatus func(context.Context, resource.CanReference, client.Reader) ([]resource.ReferenceStatus, error)
	MockBuild     func(context.Context, resource.CanReference, client.Reader) (string, error)
	MockAssign    func(resource.CanReference, string) error
}

func (m *mockReferencer) GetStatus(ctx context.Context, res resource.CanReference, c client.Reader) ([]resource.ReferenceStatus, error) {
	return m.MockGetStatus(ctx, res, c)
}

func (m *mockReferencer) Build(ctx context.Context, res resource.CanReference, c client.Reader) (string, error) {
	return m.MockBuild(ctx, res, c)
}

func (m *mockReferencer) Assign(res resource.CanReference, value string) error {
	return m.MockAssign(res, value)
}

func TestResolveReferences(t *testing.T) {
	errBoom := errors.New("boom")
	wantValue := "built"

	type args struct {
		ctx context.Context
		res resource.CanReference
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		o      []APIReferenceResolverOption
		args   args
		want   error
	}{
		"NoReferencersFound": {
			reason: "Should return early without error when no referencers are found.",
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
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
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
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
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return []resource.ReferenceStatus{{Status: resource.ReferenceNotReady}}, nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
			},
			want: &referencesAccessErr{statuses: []resource.ReferenceStatus{{Status: resource.ReferenceNotReady}}},
		},
		"BuildValueError": {
			reason: "Should return an error when a referencer.Build returns an error.",
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ resource.CanReference, _ client.Reader) (string, error) {
								return "", errBoom
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errBuildAttribute),
		},
		"AssignValueError": {
			reason: "Should return an error when a referencer.Assign returns an error.",
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ resource.CanReference, _ client.Reader) (string, error) {
								return "", nil
							},
							MockAssign: func(_ resource.CanReference, _ string) error {
								return errBoom
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errAssignAttribute),
		},
		"SuccessfulNoop": {
			reason: "Should return without error when assignment does not change resource.CanReference.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ resource.CanReference, _ client.Reader) (string, error) {
								return wantValue, nil
							},
							MockAssign: func(res resource.CanReference, gotValue string) error {
								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &fake.Managed{},
			},
			want: nil,
		},
		"SuccessfulUpdate": {
			reason: "Should return without error when a value is successfully built and assigned.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ resource.CanReference, _ client.Reader) (string, error) {
								return wantValue, nil
							},
							MockAssign: func(res resource.CanReference, gotValue string) error {
								if diff := cmp.Diff(wantValue, gotValue); diff != "" {
									reason := "referencer.Assign should be called with the value returned by referencer.Build."
									t.Errorf("\nReason: %s\nreferencer.Assign(...):\n%s", reason, diff)
								}

								// Simulate assignment by changing something about the resource.
								res.(*fake.Managed).SetAnnotations(map[string]string{"assigned": "true"})
								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &fake.Managed{},
			},
			want: nil,
		},
		"Updateresource.CanReferenceError": {
			reason: "Should return an error when resource.CanReference cannot be updated.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockReferencer{
							MockGetStatus: func(_ context.Context, _ resource.CanReference, _ client.Reader) ([]resource.ReferenceStatus, error) {
								return nil, nil
							},
							MockBuild: func(_ context.Context, _ resource.CanReference, _ client.Reader) (string, error) {
								return "", nil
							},
							MockAssign: func(res resource.CanReference, _ string) error {
								// Simulate assignment by changing something about the resource.
								res.(*fake.Managed).SetAnnotations(map[string]string{"assigned": "true"})

								return nil
							},
						},
					}
				})),
			},
			args: args{
				ctx: context.Background(),
				res: &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errUpdateReferencer),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIReferenceResolver(tc.c, tc.o...)
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
		want   []resource.AttributeReferencer
	}{
		"ObjIsNil": {
			reason: "The root object is nil, and therefore should not satisfy resource.AttributeReferencer.",
		},
		"ObjIsNilresource.AttributeReferencer": {
			reason: "The root object satisfies resource.AttributeReferencer, but is nil and thus presumed unsafe to call.",
			obj:    (*mockReferencer)(nil),
		},
		"ObjIsAttributeReferencer": {
			reason: "The root object should satisfy resource.AttributeReferencer.",
			obj:    &mockReferencer{},
			want:   []resource.AttributeReferencer{&mockReferencer{}},
		},
		"FieldIsAttributeReferencer": {
			reason: "The root is a struct with a field object should satisfy resource.AttributeReferencer.",
			obj: struct {
				Referencer *mockReferencer
			}{
				Referencer: &mockReferencer{},
			},
			want: []resource.AttributeReferencer{&mockReferencer{}},
		},
		"FieldInPointerToStructIsAttributeReferencer": {
			reason: "The root object is a pointer to struct with a field that should satisfy resource.AttributeReferencer.",
			obj: func() interface{} {
				obj := struct {
					Referencer *mockReferencer
				}{
					Referencer: &mockReferencer{},
				}
				return &obj
			}(),
			want: []resource.AttributeReferencer{&mockReferencer{}},
		},
		"FieldIsNotresource.AttributeReferencer": {
			reason: "The root object is a struct whose only field should not satisfy resource.AttributeReferencer.",
			obj: struct {
				Unused string
			}{
				Unused: "notareferencer",
			},
			want: []resource.AttributeReferencer{},
		},
		"FieldIsNotExported": {
			reason: "The root object is a struct whose only field satisfies resource.AttributeReferencer, but is not exported and should thus fail CanInterface.",
			obj: struct {
				referencer *mockReferencer
			}{
				referencer: &mockReferencer{},
			},
			want: []resource.AttributeReferencer{},
		},
		"ElementIsAttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should satisfy resource.AttributeReferencer.",
			obj: struct {
				Referencers []resource.AttributeReferencer
			}{
				Referencers: []resource.AttributeReferencer{&mockReferencer{}},
			},
			want: []resource.AttributeReferencer{&mockReferencer{}},
		},
		"ElementIsNotresource.AttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should not satisfy resource.AttributeReferencer.",
			obj: struct {
				Unused []string
			}{
				Unused: []string{"notareferencer"},
			},
			want: []resource.AttributeReferencer{},
		},
		"MockManagedIsNotresource.AttributeReferencer": {
			reason: "Managed is relatively complex, but should not break findReferencers",
			obj:    &fake.Managed{},
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
