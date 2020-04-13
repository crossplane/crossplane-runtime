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

type mockSelector struct {
	MockSelect func(context.Context, client.Reader, resource.Managed) error
}

func (m *mockSelector) Select(ctx context.Context, c client.Reader, mg resource.Managed) error {
	return m.MockSelect(ctx, c, mg)
}

type mockReferencer struct {
	MockResolve func(context.Context, client.Reader, resource.Managed) error
}

func (m *mockReferencer) Resolve(ctx context.Context, c client.Reader, mg resource.Managed) error {
	return m.MockResolve(ctx, c, mg)
}

type mockAttributeReferencer struct {
	MockGetStatus func(context.Context, resource.CanReference, client.Reader) ([]resource.ReferenceStatus, error)
	MockBuild     func(context.Context, resource.CanReference, client.Reader) (string, error)
	MockAssign    func(resource.CanReference, string) error
}

func (m *mockAttributeReferencer) GetStatus(ctx context.Context, res resource.CanReference, c client.Reader) ([]resource.ReferenceStatus, error) {
	return m.MockGetStatus(ctx, res, c)
}

func (m *mockAttributeReferencer) Build(ctx context.Context, res resource.CanReference, c client.Reader) (string, error) {
	return m.MockBuild(ctx, res, c)
}

func (m *mockAttributeReferencer) Assign(res resource.CanReference, value string) error {
	return m.MockAssign(res, value)
}

func TestResolveReferences(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		o      []APIReferenceResolverOption
		args   args
		want   error
	}{
		"NoReferencersFound": {
			reason: "Should return without error when no selectors or referencers are found.",
			o: []APIReferenceResolverOption{
				WithSelectorFinder(SelectorFinderFn(func(_ interface{}) []Selector { return nil })),
				WithReferencerFinder(ReferencerFinderFn(func(_ interface{}) []Referencer { return nil })),
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer { return nil })),
			},
			args: args{
				mg:  &fake.Managed{},
				ctx: context.Background(),
			},
			want: nil,
		},
		"SelectError": {
			reason: "Should return errors encountered during reference selection.",
			o: []APIReferenceResolverOption{
				WithSelectorFinder(SelectorFinderFn(func(_ interface{}) []Selector {
					return []Selector{&mockSelector{MockSelect: func(context.Context, client.Reader, resource.Managed) error { return errBoom }}}
				})),
				WithReferencerFinder(ReferencerFinderFn(func(_ interface{}) []Referencer { return nil })),
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer { return nil })),
			},
			args: args{
				mg:  &fake.Managed{},
				ctx: context.Background(),
			},
			want: errors.Wrap(errBoom, errSelectReference),
		},
		"ResolveError": {
			reason: "Should return errors encountered during reference resolution.",
			o: []APIReferenceResolverOption{
				WithSelectorFinder(SelectorFinderFn(func(_ interface{}) []Selector { return nil })),
				WithReferencerFinder(ReferencerFinderFn(func(_ interface{}) []Referencer {
					return []Referencer{&mockReferencer{MockResolve: func(context.Context, client.Reader, resource.Managed) error { return errBoom }}}
				})),
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer { return nil })),
			},
			args: args{
				mg:  &fake.Managed{},
				ctx: context.Background(),
			},
			want: errors.Wrap(errBoom, errResolveReference),
		},
		"UpdateError": {
			reason: "Should return errors encountered while updating the managed resource.",
			c:      &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			o: []APIReferenceResolverOption{
				WithSelectorFinder(SelectorFinderFn(func(_ interface{}) []Selector {
					return []Selector{&mockSelector{MockSelect: func(_ context.Context, _ client.Reader, mg resource.Managed) error {
						// Change something about the managed resource.
						mg.SetName("I'm different!")
						return nil
					}}}
				})),
				WithReferencerFinder(ReferencerFinderFn(func(_ interface{}) []Referencer { return nil })),
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer { return nil })),
			},
			args: args{
				mg:  &fake.Managed{},
				ctx: context.Background(),
			},
			want: errors.Wrap(errBoom, errUpdateManaged),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIReferenceResolver(tc.c, tc.o...)
			got := r.ResolveReferences(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\r.ResolveReferences(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolveAttributeReferences(t *testing.T) {
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
						&mockAttributeReferencer{
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
						&mockAttributeReferencer{
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
						&mockAttributeReferencer{
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
						&mockAttributeReferencer{
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
		"SuccessfulUpdate": {
			reason: "Should return without error when a value is successfully built and assigned.",
			o: []APIReferenceResolverOption{
				WithAttributeReferencerFinder(AttributeReferencerFinderFn(func(_ interface{}) []resource.AttributeReferencer {
					return []resource.AttributeReferencer{
						&mockAttributeReferencer{
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPIReferenceResolver(tc.c, tc.o...)
			got := r.resolveAttributeReferences(tc.args.ctx, tc.args.res)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\r.resolveAttributeReferences(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFindSelectors(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    interface{}
		want   []Selector
	}{
		"ObjIsNil": {
			reason: "The root object is nil, and therefore should not satisfy Selector.",
		},
		"ObjIsNilSelector": {
			reason: "The root object satisfies Selector, but is nil and thus presumed unsafe to call.",
			obj:    (*mockSelector)(nil),
		},
		"ObjIsSelector": {
			reason: "The root object should satisfy Selector.",
			obj:    &mockSelector{},
			want:   []Selector{&mockSelector{}},
		},
		"FieldIsSelector": {
			reason: "The root is a struct with a field object should satisfy Selector.",
			obj: struct {
				Selector *mockSelector
			}{
				Selector: &mockSelector{},
			},
			want: []Selector{&mockSelector{}},
		},
		"FieldInPointerToStructIsSelector": {
			reason: "The root object is a pointer to struct with a field that should satisfy Selector.",
			obj: func() interface{} {
				obj := struct {
					Selector *mockSelector
				}{
					Selector: &mockSelector{},
				}
				return &obj
			}(),
			want: []Selector{&mockSelector{}},
		},
		"FieldIsNotSelector": {
			reason: "The root object is a struct whose only field should not satisfy Selector.",
			obj: struct {
				Unused string
			}{
				Unused: "notaselector",
			},
			want: []Selector{},
		},
		"FieldIsNotExported": {
			reason: "The root object is a struct whose only field satisfies Selector, but is not exported and should thus fail CanInterface.",
			obj: struct {
				referencer *mockSelector
			}{
				referencer: &mockSelector{},
			},
			want: []Selector{},
		},
		"ElementIsSelector": {
			reason: "The root object is a struct whose only field is a slice of elements that should satisfy Selector.",
			obj: struct {
				Selectors []Selector
			}{
				Selectors: []Selector{&mockSelector{}},
			},
			want: []Selector{&mockSelector{}},
		},
		"ElementIsNotSelector": {
			reason: "The root object is a struct whose only field is a slice of elements that should not satisfy Selector.",
			obj: struct {
				Unused []string
			}{
				Unused: []string{"notaselector"},
			},
			want: []Selector{},
		},
		"MockManagedIsNotSelector": {
			reason: "Managed is relatively complex, but should not break findSelectors",
			obj:    &fake.Managed{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := findSelectors(tc.obj)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\nReason: %s\nfindSelectors(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFindReferencers(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    interface{}
		want   []Referencer
	}{
		"ObjIsNil": {
			reason: "The root object is nil, and therefore should not satisfy Referencer.",
		},
		"ObjIsNilReferencer": {
			reason: "The root object satisfies Referencer, but is nil and thus presumed unsafe to call.",
			obj:    (*mockReferencer)(nil),
		},
		"ObjIsReferencer": {
			reason: "The root object should satisfy Referencer.",
			obj:    &mockReferencer{},
			want:   []Referencer{&mockReferencer{}},
		},
		"FieldIsReferencer": {
			reason: "The root is a struct with a field object should satisfy Referencer.",
			obj: struct {
				Referencer *mockReferencer
			}{
				Referencer: &mockReferencer{},
			},
			want: []Referencer{&mockReferencer{}},
		},
		"FieldInPointerToStructIsReferencer": {
			reason: "The root object is a pointer to struct with a field that should satisfy Referencer.",
			obj: func() interface{} {
				obj := struct {
					Referencer *mockReferencer
				}{
					Referencer: &mockReferencer{},
				}
				return &obj
			}(),
			want: []Referencer{&mockReferencer{}},
		},
		"FieldIsNotReferencer": {
			reason: "The root object is a struct whose only field should not satisfy Referencer.",
			obj: struct {
				Unused string
			}{
				Unused: "notareferencer",
			},
			want: []Referencer{},
		},
		"FieldIsNotExported": {
			reason: "The root object is a struct whose only field satisfies Referencer, but is not exported and should thus fail CanInterface.",
			obj: struct {
				referencer *mockReferencer
			}{
				referencer: &mockReferencer{},
			},
			want: []Referencer{},
		},
		"ElementIsReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should satisfy Referencer.",
			obj: struct {
				Referencers []Referencer
			}{
				Referencers: []Referencer{&mockReferencer{}},
			},
			want: []Referencer{&mockReferencer{}},
		},
		"ElementIsNotReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should not satisfy Referencer.",
			obj: struct {
				Unused []string
			}{
				Unused: []string{"notareferencer"},
			},
			want: []Referencer{},
		},
		"MockManagedIsNotReferencer": {
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

func TestFindAttributeReferencers(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    interface{}
		want   []resource.AttributeReferencer
	}{
		"ObjIsNil": {
			reason: "The root object is nil, and therefore should not satisfy resource.AttributeReferencer.",
		},
		"ObjIsNilAttributeReferencer": {
			reason: "The root object satisfies resource.AttributeReferencer, but is nil and thus presumed unsafe to call.",
			obj:    (*mockAttributeReferencer)(nil),
		},
		"ObjIsAttributeReferencer": {
			reason: "The root object should satisfy resource.AttributeReferencer.",
			obj:    &mockAttributeReferencer{},
			want:   []resource.AttributeReferencer{&mockAttributeReferencer{}},
		},
		"FieldIsAttributeReferencer": {
			reason: "The root is a struct with a field object should satisfy resource.AttributeReferencer.",
			obj: struct {
				Referencer *mockAttributeReferencer
			}{
				Referencer: &mockAttributeReferencer{},
			},
			want: []resource.AttributeReferencer{&mockAttributeReferencer{}},
		},
		"FieldInPointerToStructIsAttributeReferencer": {
			reason: "The root object is a pointer to struct with a field that should satisfy resource.AttributeReferencer.",
			obj: func() interface{} {
				obj := struct {
					Referencer *mockAttributeReferencer
				}{
					Referencer: &mockAttributeReferencer{},
				}
				return &obj
			}(),
			want: []resource.AttributeReferencer{&mockAttributeReferencer{}},
		},
		"FieldIsNotAttributeReferencer": {
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
				referencer *mockAttributeReferencer
			}{
				referencer: &mockAttributeReferencer{},
			},
			want: []resource.AttributeReferencer{},
		},
		"ElementIsAttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should satisfy resource.AttributeReferencer.",
			obj: struct {
				Referencers []resource.AttributeReferencer
			}{
				Referencers: []resource.AttributeReferencer{&mockAttributeReferencer{}},
			},
			want: []resource.AttributeReferencer{&mockAttributeReferencer{}},
		},
		"ElementIsNotAttributeReferencer": {
			reason: "The root object is a struct whose only field is a slice of elements that should not satisfy resource.AttributeReferencer.",
			obj: struct {
				Unused []string
			}{
				Unused: []string{"notareferencer"},
			},
			want: []resource.AttributeReferencer{},
		},
		"MockManagedIsNotAttributeReferencer": {
			reason: "Managed is relatively complex, but should not break findReferencers",
			obj:    &fake.Managed{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := findAttributeReferencers(tc.obj)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\nReason: %s\nfindReferencers(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
