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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var (
	_ Initializer              = &NameAsExternalName{}
	_ ConnectionPublisher      = &APISecretPublisher{}
	_ LocalConnectionPublisher = &APILocalSecretPublisher{}
)

func TestNameAsExternalName(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")
	testExternalName := "my-" +
		"external-name"

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: testExternalName},
				}},
			},
		},
		"UpdateSuccessful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: nil,
				mg: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: testExternalName},
				}},
			},
		},
		"UpdateNotNeeded": {
			args: args{
				ctx: context.Background(),
				mg: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: "some-name"},
				}},
			},
			want: want{
				err: nil,
				mg: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: "some-name"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewNameAsExternalName(tc.client)

			err := api.Initialize(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPISecretPublisher(t *testing.T) {
	errBoom := errors.New("boom")

	mg := &fake.LegacyManaged{
		ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &xpv2.SecretReference{
			Namespace: "coolnamespace",
			Name:      "coolsecret",
		}},
	}

	cd := ConnectionDetails{"cool": {42}}

	type fields struct {
		secret resource.Applicator
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		mg  resource.LegacyManaged
		c   ConnectionDetails
	}

	type want struct {
		err       error
		published bool
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ResourceDoesNotPublishSecret": {
			reason: "A managed resource with a nil GetWriteConnectionSecretToReference should not publish a secret",
			args: args{
				ctx: context.Background(),
				mg:  &fake.LegacyManaged{},
			},
		},
		"ApplyError": {
			reason: "An error applying the connection secret should be returned",
			fields: fields{
				secret: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error { return errBoom }),
				typer:  fake.SchemeWith(&fake.LegacyManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateOrUpdateSecret),
			},
		},
		"AlreadyPublished": {
			reason: "An up to date connection secret should result in no error and not being published",
			fields: fields{
				secret: resource.ApplyFn(func(ctx context.Context, o client.Object, ao ...resource.ApplyOption) error {
					want := resource.ConnectionSecretFor(mg, fake.GVK(mg))

					want.Data = cd
					for _, fn := range ao {
						if err := fn(ctx, o, want); err != nil {
							return err
						}
					}

					return nil
				}),
				typer: fake.SchemeWith(&fake.LegacyManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
				c:   cd,
			},
			want: want{
				published: false,
				err:       nil,
			},
		},
		"Success": {
			reason: "A successful application of the connection secret should result in no error",
			fields: fields{
				secret: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
					want := resource.ConnectionSecretFor(mg, fake.GVK(mg))

					want.Data = cd
					if diff := cmp.Diff(want, o); diff != "" {
						t.Errorf("-want, +got:\n%s", diff)
					}

					return nil
				}),
				typer: fake.SchemeWith(&fake.LegacyManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
				c:   cd,
			},
			want: want{
				published: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &APISecretPublisher{tc.fields.secret, tc.fields.typer}

			got, gotErr := a.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPublish(...): -wantErr, +gotErr:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.published, got); diff != "" {
				t.Errorf("\n%s\nPublish(...): -wantPublished, +gotPublished:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPILocalSecretPublisher(t *testing.T) {
	errBoom := errors.New("boom")

	mg := &fake.ModernManaged{
		LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{Ref: &xpv2.LocalSecretReference{
			Name: "coolsecret",
		}},
	}

	cd := ConnectionDetails{"cool": {42}}

	type fields struct {
		secret resource.Applicator
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		mg  resource.ModernManaged
		c   ConnectionDetails
	}

	type want struct {
		err       error
		published bool
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"ResourceDoesNotPublishSecret": {
			reason: "A managed resource with a nil GetWriteConnectionSecretToReference should not publish a secret",
			args: args{
				ctx: context.Background(),
				mg:  &fake.ModernManaged{},
			},
		},
		"ApplyError": {
			reason: "An error applying the connection secret should be returned",
			fields: fields{
				secret: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error { return errBoom }),
				typer:  fake.SchemeWith(&fake.ModernManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateOrUpdateSecret),
			},
		},
		"AlreadyPublished": {
			reason: "An up to date connection secret should result in no error and not being published",
			fields: fields{
				secret: resource.ApplyFn(func(ctx context.Context, o client.Object, ao ...resource.ApplyOption) error {
					want := resource.LocalConnectionSecretFor(mg, fake.GVK(mg))

					want.Data = cd
					for _, fn := range ao {
						if err := fn(ctx, o, want); err != nil {
							return err
						}
					}

					return nil
				}),
				typer: fake.SchemeWith(&fake.ModernManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
				c:   cd,
			},
			want: want{
				published: false,
				err:       nil,
			},
		},
		"Success": {
			reason: "A successful application of the connection secret should result in no error",
			fields: fields{
				secret: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
					want := resource.LocalConnectionSecretFor(mg, fake.GVK(mg))

					want.Data = cd
					if diff := cmp.Diff(want, o); diff != "" {
						t.Errorf("-want, +got:\n%s", diff)
					}

					return nil
				}),
				typer: fake.SchemeWith(&fake.ModernManaged{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
				c:   cd,
			},
			want: want{
				published: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &APILocalSecretPublisher{tc.fields.secret, tc.fields.typer}

			got, gotErr := a.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPublish(...): -wantErr, +gotErr:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.published, got); diff != "" {
				t.Errorf("\n%s\nPublish(...): -wantPublished, +gotPublished:\n%s", tc.reason, diff)
			}
		})
	}
}

type mockSimpleReferencer struct {
	resource.Managed

	MockResolveReferences func(context.Context, client.Reader) error `json:"-"`
}

func (r *mockSimpleReferencer) ResolveReferences(ctx context.Context, c client.Reader) error {
	return r.MockResolveReferences(ctx, c)
}

func (r *mockSimpleReferencer) DeepCopyObject() runtime.Object {
	return &mockSimpleReferencer{Managed: r.Managed.DeepCopyObject().(resource.Managed)}
}

func (r *mockSimpleReferencer) Equal(s *mockSimpleReferencer) bool {
	return cmp.Equal(r.Managed, s.Managed)
}

// mockStructuredReferencer embeds the fake managed resource instead of wrapping
// it, so that it marshals to the managed resource's own JSON shape. The field
// ownership tests rely on that shape to address fields from managedFields.
type mockStructuredReferencer struct {
	fake.LegacyManaged

	MockResolveReferences func(context.Context, client.Reader) error `json:"-"`
}

func (r *mockStructuredReferencer) ResolveReferences(ctx context.Context, c client.Reader) error {
	return r.MockResolveReferences(ctx, c)
}

func (r *mockStructuredReferencer) DeepCopyObject() runtime.Object {
	out := *r
	out.LegacyManaged = *r.LegacyManaged.DeepCopyObject().(*fake.LegacyManaged)

	return &out
}

func (r *mockStructuredReferencer) Equal(s *mockStructuredReferencer) bool {
	return cmp.Equal(r.LegacyManaged, s.LegacyManaged)
}

func ownedBy(manager string, op metav1.ManagedFieldsOperationType, fieldsV1 string) []metav1.ManagedFieldsEntry {
	return []metav1.ManagedFieldsEntry{{
		Manager:    manager,
		Operation:  op,
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(fieldsV1)},
	}}
}

func TestResolveReferences(t *testing.T) {
	errBoom := errors.New("boom")

	different := &fake.LegacyManaged{}

	owned := &mockStructuredReferencer{
		LegacyManaged: fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
			Name:          "owned",
			Annotations:   map[string]string{"resolved-a": "1"},
			ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationApply, `{"f:annotations":{"f:resolved-a":{}}}`),
		}},
	}
	owned.MockResolveReferences = func(context.Context, client.Reader) error {
		owned.Annotations["resolved-b"] = "2"
		return nil
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   error
	}{
		"NoReferencersFound": {
			reason: "Should return early without error when the managed resource has no references.",
			args: args{
				ctx: context.Background(),
				mg:  &fake.LegacyManaged{},
			},
			want: nil,
		},
		"ResolveReferencesError": {
			reason: "Should return errors encountered while resolving references.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &mockSimpleReferencer{
					Managed: &fake.LegacyManaged{},
					MockResolveReferences: func(context.Context, client.Reader) error {
						return errBoom
					},
				},
			},
			want: errors.Wrap(errBoom, errResolveReferences),
		},
		"SuccessfulNoop": {
			reason: "Should return without error when resolution does not change the managed resource.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &mockSimpleReferencer{
					Managed: &fake.LegacyManaged{},
					MockResolveReferences: func(context.Context, client.Reader) error {
						return nil
					},
				},
			},
			want: nil,
		},
		"SuccessfulUpdate": {
			reason: "Should return without error when a value is successfully resolved.",
			c: &test.MockClient{
				MockPatch: test.NewMockPatchFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &mockSimpleReferencer{
					Managed: different,
					MockResolveReferences: func(context.Context, client.Reader) error {
						different.SetName("I'm different!")
						return nil
					},
				},
			},
			want: nil,
		},
		"PatchKeepsOwnedFields": {
			reason: "Should keep the fields the resolver already owns in the server-side apply document next to the newly resolved ones.",
			c: &test.MockClient{
				MockPatch: func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
					if patch.Type() != types.ApplyPatchType {
						return errors.Errorf("unexpected patch type %q", patch.Type())
					}

					got, err := patch.Data(obj)
					if err != nil {
						return err
					}

					if diff := cmp.Diff(`{"annotations":{"resolved-a":"1","resolved-b":"2"}}`, string(got)); diff != "" {
						return errors.Errorf("unexpected patch: -want, +got:\n%s", diff)
					}

					return nil
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  owned,
			},
			want: nil,
		},
		"PatchError": {
			reason: "Should return an error when the managed resource cannot be updated.",
			c: &test.MockClient{
				MockPatch: test.NewMockPatchFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &mockSimpleReferencer{
					Managed: different,
					MockResolveReferences: func(context.Context, client.Reader) error {
						different.SetName("I'm different-er!")
						return nil
					},
				},
			},
			want: errors.Wrap(errBoom, errPatchManaged),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPISimpleReferenceResolver(tc.c)

			got := r.ResolveReferences(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.ResolveReferences(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPrepareJSONMerge(t *testing.T) {
	type args struct {
		existing runtime.Object
		resolved runtime.Object
	}

	type want struct {
		patch string
		err   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulPatch": {
			reason: "Should successfully compute the JSON merge patch document.",
			args: args{
				existing: &fake.LegacyManaged{},
				resolved: &fake.LegacyManaged{
					ObjectMeta: metav1.ObjectMeta{
						Name: "resolved",
					},
				},
			},
			want: want{
				patch: `{"name":"resolved"}`,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			patch, err := prepareJSONMerge(tc.args.existing, tc.args.resolved)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nprepareJSONMerge(...): -wantErr, +gotErr:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.patch, string(patch)); diff != "" {
				t.Errorf("\n%s\nprepareJSONMerge(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWithOwnedFields(t *testing.T) {
	type args struct {
		existing runtime.Object
		patch    string
	}

	type want struct {
		patch string
		err   error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoManagedFields": {
			reason: "Should return the patch unchanged when the object records no field ownership.",
			args: args{
				existing: &fake.LegacyManaged{},
				patch:    `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"b":"2"}}`},
		},
		"OtherManager": {
			reason: "Should ignore fields owned by other field managers.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Annotations:   map[string]string{"a": "1"},
					ManagedFields: ownedBy("someone-else", metav1.ManagedFieldsOperationApply, `{"f:annotations":{"f:a":{}}}`),
				}},
				patch: `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"b":"2"}}`},
		},
		"UpdateOperationIgnored": {
			reason: "Should only consider fields the resolver owns through an apply operation.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Annotations:   map[string]string{"a": "1"},
					ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationUpdate, `{"f:annotations":{"f:a":{}}}`),
				}},
				patch: `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"b":"2"}}`},
		},
		"OwnedFieldsKept": {
			reason: "Should carry the fields the resolver already owns along with the newly resolved ones.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Annotations:   map[string]string{"a": "1", "unowned": "x"},
					ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationApply, `{"f:annotations":{"f:a":{}}}`),
				}},
				patch: `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"a":"1","b":"2"}}`},
		},
		"ResolvedValueWins": {
			reason: "Should prefer the value resolved now over the value recorded on the object for a field the resolver owns.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Annotations:   map[string]string{"a": "1"},
					ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationApply, `{"f:annotations":{"f:a":{}}}`),
				}},
				patch: `{"annotations":{"a":"9"}}`,
			},
			want: want{patch: `{"annotations":{"a":"9"}}`},
		},
		"OwnedListKept": {
			reason: "Should carry a list the resolver owns as a whole.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Finalizers:    []string{"a", "b"},
					ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationApply, `{"f:finalizers":{}}`),
				}},
				patch: `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"b":"2"},"finalizers":["a","b"]}`},
		},
		"OwnedFieldMissingFromObject": {
			reason: "Should tolerate ownership of a field the object no longer has.",
			args: args{
				existing: &fake.LegacyManaged{ObjectMeta: metav1.ObjectMeta{
					Annotations:   map[string]string{"a": "1"},
					ManagedFields: ownedBy(fieldOwnerAPISimpleRefResolver, metav1.ManagedFieldsOperationApply, `{"f:annotations":{"f:gone":{}},"f:labels":{"f:gone":{}}}`),
				}},
				patch: `{"annotations":{"b":"2"}}`,
			},
			want: want{patch: `{"annotations":{"b":"2"}}`},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			patch, err := withOwnedFields(tc.args.existing, []byte(tc.args.patch))
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nwithOwnedFields(...): -wantErr, +gotErr:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.patch, string(patch)); diff != "" {
				t.Errorf("\n%s\nwithOwnedFields(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRetryingCriticalAnnotationUpdater(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		o   client.Object
	}

	type want struct {
		err error
		o   client.Object
	}

	setLabels := func(obj client.Object) error {
		obj.SetLabels(map[string]string{"getcalled": "true"})
		return nil
	}
	objectReturnedByGet := &fake.LegacyManaged{}
	setLabels(objectReturnedByGet)

	cases := map[string]struct {
		reason string
		c      *test.MockClient
		args   args
		want   want
	}{
		"UpdateConflictGetError": {
			reason: "We should return any error we encounter getting the supplied object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom, setLabels),
				MockUpdate: test.NewMockUpdateFn(kerrors.NewConflict(schema.GroupResource{
					Group:    "foo.com",
					Resource: "bars",
				}, "abc", errBoom)),
			},
			args: args{
				o: &fake.LegacyManaged{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateCriticalAnnotations),
				o:   objectReturnedByGet,
			},
		},
		"UpdateError": {
			reason: "We should return any error we encounter updating the supplied object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(nil, setLabels),
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				o: &fake.LegacyManaged{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateCriticalAnnotations),
				o:   &fake.LegacyManaged{},
			},
		},
		"SuccessfulGetAfterAConflict": {
			reason: "A successful get after a conflict should not hide the conflict error and prevent retries",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, setLabels),
				MockUpdate: test.NewMockUpdateFn(kerrors.NewConflict(schema.GroupResource{
					Group:    "foo.com",
					Resource: "bars",
				}, "abc", errBoom)),
			},
			args: args{
				o: &fake.LegacyManaged{},
			},
			want: want{
				err: errors.Wrap(kerrors.NewConflict(schema.GroupResource{
					Group:    "foo.com",
					Resource: "bars",
				}, "abc", errBoom), errUpdateCriticalAnnotations),
				o: objectReturnedByGet,
			},
		},
		"Success": {
			reason: "We should return without error if we successfully update our annotations",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(nil, setLabels),
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				o: &fake.LegacyManaged{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateCriticalAnnotations),
				o:   &fake.LegacyManaged{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			u := NewRetryingCriticalAnnotationUpdater(tc.c)

			got := u.UpdateCriticalAnnotations(tc.args.ctx, tc.args.o)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nu.UpdateCriticalAnnotations(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nu.UpdateCriticalAnnotations(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
