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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ConnectionPropagator        = &APIConnectionPropagator{}
	_ ManagedConnectionPropagator = &APIManagedConnectionPropagator{}
)

func TestPropagateConnection(t *testing.T) {
	errBoom := errors.New("boom")

	mgcsns := "coolnamespace"
	mgcsname := "coolmanagedsecret"
	mgcsdata := map[string][]byte{"cool": {1}}

	cmcsns := "coolnamespace"
	cmcsname := "coolclaimsecret"

	mg := &fake.Managed{
		ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
			Ref: &xpv1.SecretReference{Namespace: mgcsns, Name: mgcsname},
		},
	}

	cm := &fake.CompositeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: cmcsns},
		LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
			Ref: &xpv1.LocalSecretReference{Name: cmcsname},
		},
	}

	type fields struct {
		client ClientApplicator
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		o   LocalConnectionSecretOwner
		mg  Managed
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"ClaimDoesNotWantConnectionSecret": {
			reason: "The managed resource's secret should not be propagated if the claim does not want to write one",
			args: args{
				o:  &fake.CompositeClaim{},
				mg: mg,
			},
			want: nil,
		},
		"ManagedDoesNotExposeConnectionSecret": {
			reason: "The managed resource's secret should not be propagated if it does not have one",
			args: args{
				o:  cm,
				mg: &fake.Managed{},
			},
			want: nil,
		},
		"GetManagedSecretError": {
			reason: "Errors getting the managed resource's connection secret should be returned",
			fields: fields{
				client: ClientApplicator{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				},
			},
			args: args{
				o:  cm,
				mg: mg,
			},
			want: errors.Wrap(errBoom, errGetSecret),
		},
		"ManagedResourceDoesNotControlSecret": {
			reason: "The managed resource must control its connection secret before it can be propagated",
			fields: fields{
				client: ClientApplicator{
					// Simulate getting a secret that is not controlled by the
					// managed resource by not modifying the secret passed to
					// the client, and not returning an error. We thus proceed
					// with our original empty secret, which has no controller
					// reference.
					Client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
				},
			},
			args: args{
				o:  cm,
				mg: mg,
			},
			want: errors.New(errSecretConflict),
		},
		"ApplyClaimSecretError": {
			reason: "Errors applying the claim connection secret should be returned",
			fields: fields{
				client: ClientApplicator{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						s := ConnectionSecretFor(mg, fake.GVK(mg))
						*o.(*corev1.Secret) = *s
						return nil
					})},
					Applicator: ApplyFn(func(_ context.Context, _ client.Object, _ ...ApplyOption) error { return errBoom }),
				},
				typer: fake.SchemeWith(mg, cm),
			},
			args: args{
				o:  cm,
				mg: mg,
			},
			want: errors.Wrap(errBoom, errCreateOrUpdateSecret),
		},
		"UpdateManagedSecretError": {
			reason: "Errors updating the managed resource connection secret should be returned",
			fields: fields{
				client: ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							s := ConnectionSecretFor(mg, fake.GVK(mg))
							*o.(*corev1.Secret) = *s
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Applicator: ApplyFn(func(_ context.Context, _ client.Object, _ ...ApplyOption) error { return nil }),
				},
				typer: fake.SchemeWith(mg, cm),
			},
			args: args{
				o:  cm,
				mg: mg,
			},
			want: errors.Wrap(errBoom, errUpdateSecret),
		},
		"Successful": {
			reason: "Successful propagation should update the claim and managed resource secrets with the appropriate values",
			fields: fields{
				client: ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// The managed secret has some data when we get it.
							s := ConnectionSecretFor(mg, fake.GVK(mg))
							s.Data = mgcsdata

							*o.(*corev1.Secret) = *s
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
							// Ensure the managed secret is annotated to allow
							// constant propagation to the claim secret.
							want := ConnectionSecretFor(mg, fake.GVK(mg))
							want.Data = mgcsdata
							meta.AllowPropagation(want, LocalConnectionSecretFor(cm, fake.GVK(cm)))
							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got: %s", diff)
							}
							return nil
						}),
					},
					Applicator: ApplyFn(func(_ context.Context, o client.Object, _ ...ApplyOption) error {
						// Ensure the managed secret's data is copied to the
						// claim secret, and that the claim secret is annotated
						// to allow constant propagation from the managed
						// secret.
						want := LocalConnectionSecretFor(cm, fake.GVK(cm))
						want.Data = mgcsdata
						meta.AllowPropagation(ConnectionSecretFor(mg, fake.GVK(mg)), want)
						if diff := cmp.Diff(want, o); diff != "" {
							t.Errorf("-want, +got: %s", diff)
						}

						return nil
					}),
				},
				typer: fake.SchemeWith(mg, cm),
			},
			args: args{
				o:  cm,
				mg: mg,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &APIConnectionPropagator{client: tc.fields.client, typer: tc.fields.typer}
			err := api.PropagateConnection(tc.args.ctx, tc.args.o, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\napi.PropagateConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIPatchingApplicator(t *testing.T) {
	errBoom := errors.New("boom")
	desired := &object{}
	desired.SetName("desired")

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot get object"),
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot create object"),
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			args: args{
				o:  &object{},
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   &object{},
				err: errBoom,
			},
		},
		"PatchError": {
			reason: "An error should be returned if we can't patch the object",
			c: &test.MockClient{
				MockGet:   test.NewMockGetFn(nil),
				MockPatch: test.NewMockPatchFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot patch object"),
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
		"Patched": {
			reason: "No error should be returned if we successfully patch an existing object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
				MockPatch: test.NewMockPatchFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIPatchingApplicator(tc.c)
			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestAPIUpdatingApplicator(t *testing.T) {
	errBoom := errors.New("boom")
	desired := &object{}
	desired.SetName("desired")
	current := &object{}
	current.SetName("current")

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot get object"),
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot create object"),
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c:      &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			args: args{
				o:  &object{},
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   &object{},
				err: errBoom,
			},
		},
		"UpdateError": {
			reason: "An error should be returned if we can't update the object",
			c: &test.MockClient{
				MockGet:    test.NewMockGetFn(nil),
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				o: &object{},
			},
			want: want{
				o:   &object{},
				err: errors.Wrap(errBoom, "cannot update object"),
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
		"Updated": {
			reason: "No error should be returned if we successfully update an existing object. If no ApplyOption is passed the existing should not be modified",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
					if diff := cmp.Diff(*desired, *o.(*object)); diff != "" {
						t.Errorf("r: -want, +got:\n%s", diff)
					}
					return nil
				}),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIUpdatingApplicator(tc.c)
			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestManagedRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)
			err := api.RemoveFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)
			err := api.AddFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
