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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedCreator              = &APIManagedCreator{}
	_ ManagedConnectionPropagator = &APIManagedConnectionPropagator{}
	_ ManagedBinder               = &APIManagedBinder{}
	_ ManagedBinder               = &APIManagedStatusBinder{}
	_ ClaimFinalizer              = &APIClaimFinalizerRemover{}
	_ ManagedInitializer          = &APIManagedFinalizerAdder{}
	_ ManagedInitializer          = &ManagedNameAsExternalName{}
	_ ManagedFinalizer            = &APIManagedFinalizerRemover{}
)

func TestCreate(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  Claim
		cs  Class
		mg  Managed
	}

	cmname := "coolclaim"
	csname := "coolclass"
	mgname := "coolmanaged"
	errBoom := errors.New("boom")

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"CreateManagedError": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockClass{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				cs:  &MockClass{},
				mg:  &MockManaged{},
			},
			want: errors.Wrap(errBoom, errCreateManaged),
		},
		"UpdateClaimError": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockClass{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				cs:  &MockClass{},
				mg:  &MockManaged{},
			},
			want: errors.Wrap(errBoom, errUpdateClaim),
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := &MockManaged{}
						want.SetName(mgname)
						want.SetClaimReference(&corev1.ObjectReference{
							Name:       cmname,
							APIVersion: MockGVK(&MockClaim{}).GroupVersion().String(),
							Kind:       MockGVK(&MockClaim{}).Kind,
						})
						want.SetClassReference(&corev1.ObjectReference{
							Name:       csname,
							APIVersion: MockGVK(&MockClass{}).GroupVersion().String(),
							Kind:       MockGVK(&MockClass{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &MockClaim{}
						want.SetName(cmname)
						meta.AddFinalizer(want, claimFinalizerName)
						want.SetResourceReference(&corev1.ObjectReference{
							Name:       mgname,
							APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
							Kind:       MockGVK(&MockManaged{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockClass{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Name: cmname}},
				cs:  &MockClass{ObjectMeta: metav1.ObjectMeta{Name: csname}},
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Name: mgname}},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedCreator(tc.fields.client, tc.fields.typer)
			err := api.Create(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Create(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}
func TestPropagateConnection(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	cmname := "coolclaim"
	mgname := "coolmanaged"
	uid := types.UID("definitely-a-uuid")
	cmcsname := "coolclaimsecret"
	mgcsname := "coolmanagedsecret"
	mgcsdata := map[string][]byte{"cool": []byte("data")}
	controller := true
	errBoom := errors.New("boom")

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"ClaimDoesNotWantConnectionSecret": {
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				mg: &MockManaged{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: nil,
		},
		"ManagedDoesNotExposeConnectionSecret": {
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
				mg: &MockManaged{},
			},
			want: nil,
		},
		"GetManagedSecretError": {
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: cmcsname}},
				},
				mg: &MockManaged{
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: errors.Wrap(errBoom, errGetSecret),
		},
		"ClaimSecretConflictError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						switch n.Name {
						case cmcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        types.UID("some-other-uuid"),
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						case mgcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        uid,
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					ObjectMeta:                   metav1.ObjectMeta{Name: cmname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: cmcsname}},
				},
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname, UID: uid},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"ClaimSecretUncontrolledError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						switch n.Name {
						case mgcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        uid,
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						case cmcsname:
							// A secret without any owner references.
							*o.(*corev1.Secret) = corev1.Secret{}
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					ObjectMeta:                   metav1.ObjectMeta{Name: cmname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: cmcsname}},
				},
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname, UID: uid},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"ManagedSecretUpdateError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {

						switch n.Name {
						case mgcsname:
							s := corev1.Secret{}
							s.SetNamespace(namespace)
							s.SetUID(uid)
							s.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})
							s.SetName(mgcsname)
							s.Data = mgcsdata
							*o.(*corev1.Secret) = s
						case cmcsname:
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						switch got.(metav1.Object).GetName() {
						case cmcsname:
						case mgcsname:
							return errBoom
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					ObjectMeta:                   metav1.ObjectMeta{Name: cmname},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: cmcsname}},
				},
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname, UID: uid},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: errors.Wrap(errBoom, errUpdateSecret),
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := corev1.Secret{}
						s.SetNamespace(namespace)
						s.SetUID(uid)
						s.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})

						switch n.Name {
						case mgcsname:
							s.SetName(mgcsname)
							s.Data = mgcsdata
							*o.(*corev1.Secret) = s
						case cmcsname:
							s.SetName(cmcsname)
							*o.(*corev1.Secret) = s
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(namespace)
						want.SetUID(uid)
						want.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})
						want.Data = mgcsdata

						switch got.(metav1.Object).GetName() {
						case cmcsname:
							want.SetName(cmcsname)
							want.SetAnnotations(map[string]string{
								AnnotationKeyPropagateFromNamespace: namespace,
								AnnotationKeyPropagateFromName:      mgcsname,
								AnnotationKeyPropagateFromUID:       string(uid),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						case mgcsname:
							want.SetName(mgcsname)
							want.SetAnnotations(map[string]string{
								AnnotationKeyPropagateToNamespace: namespace,
								AnnotationKeyPropagateToName:      cmcsname,
								AnnotationKeyPropagateToUID:       string(uid),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					}),
				},
				typer: MockSchemeWith(&MockClaim{}, &MockManaged{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &MockClaim{
					ObjectMeta:                   metav1.ObjectMeta{Namespace: namespace, Name: cmname, UID: uid},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: cmcsname}},
				},
				mg: &MockManaged{
					ObjectMeta:                   metav1.ObjectMeta{Name: mgname, UID: uid},
					MockConnectionSecretWriterTo: MockConnectionSecretWriterTo{Ref: corev1.LocalObjectReference{Name: mgcsname}},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedConnectionPropagator(tc.fields.client, tc.fields.typer)
			err := api.PropagateConnection(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.PropagateConnection(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestBind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		cm  Claim
		mg  Managed
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
				cm:  &MockClaim{},
				mg:  &MockManaged{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				cm:  &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
				mg:  &MockManaged{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				mg:  &MockManaged{},
			},
			want: want{
				err: nil,
				cm:  &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
				mg:  &MockManaged{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedBinder(tc.client)
			err := api.Bind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Bind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Claim: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestStatusBind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		cm  Claim
		mg  Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedStatusError": {
			client: &test.MockClient{MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				mg:  &MockManaged{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				cm:  &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
				mg:  &MockManaged{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockStatusUpdate: test.NewMockStatusUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{},
				mg:  &MockManaged{},
			},
			want: want{
				err: nil,
				cm:  &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
				mg:  &MockManaged{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedStatusBinder(tc.client)
			err := api.Bind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Bind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Claim: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFinalizeResource(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"Successful": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseBound},
					MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseUnbound},
					MockClaimReferencer: MockClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseBound},
					MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseUnbound},
					MockClaimReferencer: MockClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedUnbinder(tc.client)
			err := api.Finalize(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
func TestStatusFinalizeResource(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"Successful": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseBound},
					MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &MockManaged{
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseUnbound},
					MockClaimReferencer: MockClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseBound},
					MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &MockManaged{
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseUnbound},
					MockClaimReferencer: MockClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateStatusError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseBound},
					MockClaimReferencer: MockClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				mg: &MockManaged{
					MockReclaimer:       MockReclaimer{Policy: v1alpha1.ReclaimRetain},
					MockBindable:        MockBindable{Phase: v1alpha1.BindingPhaseUnbound},
					MockClaimReferencer: MockClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedStatusUnbinder(tc.client)
			err := api.Finalize(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFinalizeClaim(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
	}

	type want struct {
		err error
		cm  Claim
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateClaimError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{claimFinalizerName}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{claimFinalizerName}}},
			},
			want: want{
				err: nil,
				cm:  &MockClaim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIClaimFinalizerRemover(tc.client)
			err := api.Finalize(tc.args.ctx, tc.args.cm)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Claim: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestFinalizeManaged(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{managedFinalizerName}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{managedFinalizerName}}},
			},
			want: want{
				err: nil,
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedFinalizerRemover(tc.client)
			err := api.Finalize(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIManagedFinalizerAdder(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{managedFinalizerName}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{managedFinalizerName}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedFinalizerAdder(tc.client)
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

func TestManagedNameAsExternalName(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &MockManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: testExternalName},
				}},
			},
		},
		"UpdateSuccessful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &MockManaged{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: nil,
				mg: &MockManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: testExternalName},
				}},
			},
		},
		"UpdateNotNeeded": {
			args: args{
				ctx: context.Background(),
				mg: &MockManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: "some-name"},
				}},
			},
			want: want{
				err: nil,
				mg: &MockManaged{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: "some-name"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewManagedNameAsExternalName(tc.client)
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
