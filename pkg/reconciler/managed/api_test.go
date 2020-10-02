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
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ Initializer = &NameAsExternalName{}
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
	testExternalName := "my-external-name"

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: testExternalName},
				}},
			},
		},
		"UpdateSuccessful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: testExternalName},
				}},
			},
		},
		"UpdateNotNeeded": {
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.AnnotationKeyExternalName: "some-name"},
				}},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
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

func TestDefaultProviderConfig(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg:  &fake.Managed{ProviderConfigReferencer: fake.ProviderConfigReferencer{Ref: &xpv1.Reference{Name: "default"}}},
			},
		},
		"UpdateSuccessful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
			},
			want: want{
				err: nil,
				mg:  &fake.Managed{ProviderConfigReferencer: fake.ProviderConfigReferencer{Ref: &xpv1.Reference{Name: "default"}}},
			},
		},
		"UpdateNotNeeded": {
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ProviderConfigReferencer: fake.ProviderConfigReferencer{Ref: &xpv1.Reference{Name: "some-value"}}},
			},
			want: want{
				err: nil,
				mg:  &fake.Managed{ProviderConfigReferencer: fake.ProviderConfigReferencer{Ref: &xpv1.Reference{Name: "some-value"}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewDefaultProviderConfig(tc.client)
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

	mg := &fake.Managed{
		ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &xpv1.SecretReference{
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
		mg  resource.Managed
		c   ConnectionDetails
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"ResourceDoesNotPublishSecret": {
			reason: "A managed resource with a nil GetWriteConnectionSecretToReference should not publish a secret",
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
			},
		},
		"ApplyError": {
			reason: "An error applying the connection secret should be returned",
			fields: fields{
				secret: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error { return errBoom }),
				typer:  fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
			},
			want: errors.Wrap(errBoom, errCreateOrUpdateSecret),
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
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg:  mg,
				c:   cd,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := &APISecretPublisher{tc.fields.secret, tc.fields.typer}
			got := a.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPublish(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type mockSimpleReferencer struct {
	resource.Managed

	MockResolveReferences func(context.Context, client.Reader) error
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

func TestResolveReferences(t *testing.T) {
	errBoom := errors.New("boom")

	different := &fake.Managed{}

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
				mg:  &fake.Managed{},
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
					Managed: &fake.Managed{},
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
					Managed: &fake.Managed{},
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
				MockUpdate: test.NewMockUpdateFn(nil),
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
		"UpdateError": {
			reason: "Should return an error when the managed resource cannot be updated.",
			c: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
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
			want: errors.Wrap(errBoom, errUpdateManaged),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewAPISimpleReferenceResolver(tc.c)
			got := r.ResolveReferences(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\r.ResolveReferences(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
