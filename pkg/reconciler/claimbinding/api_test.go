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

package claimbinding

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedCreator = &APIManagedCreator{}
	_ Binder         = &APIBinder{}
	_ Binder         = &APIStatusBinder{}
	_ ClaimFinalizer = &APIClaimFinalizer{}
)

func TestCreate(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.Class
		mg  resource.Managed
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
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				cs:  &fake.Class{},
				mg:  &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errCreateManaged),
		},
		"UpdateClaimError": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				cs:  &fake.Class{},
				mg:  &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errUpdateClaim),
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := &fake.Managed{}
						want.SetName(mgname)
						want.SetClaimReference(&corev1.ObjectReference{
							Name:       cmname,
							APIVersion: fake.GVK(&fake.Claim{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Claim{}).Kind,
						})
						want.SetClassReference(&corev1.ObjectReference{
							Name:       csname,
							APIVersion: fake.GVK(&fake.Class{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Class{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &fake.Claim{}
						want.SetName(cmname)
						want.SetResourceReference(&corev1.ObjectReference{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: cmname}},
				cs:  &fake.Class{ObjectMeta: metav1.ObjectMeta{Name: csname}},
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: mgname}},
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

func TestBind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		mg  resource.Managed
	}

	type want struct {
		err error
		cm  resource.Claim
		mg  resource.Managed
	}

	errBoom := errors.New("boom")
	externalName := "very-cool-external-resource"

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"UpdateClaimError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return nil
				case *fake.Claim:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithoutExternalName": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			typer:  fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithExternalName": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			typer:  fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIBinder(tc.client, tc.typer)
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
		cm  resource.Claim
		mg  resource.Managed
	}

	type want struct {
		err error
		cm  resource.Claim
		mg  resource.Managed
	}

	errBoom := errors.New("boom")
	externalName := "very-cool-external-resource"

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
		},
		"UpdateManagedStatusError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"UpdateClaimError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
					switch obj.(type) {
					case *fake.Managed:
						return nil
					case *fake.Claim:
						return errBoom
					default:
						return errors.New("unexpected object kind")
					}
				}),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithoutExternalName": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithExternalName": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIStatusBinder(tc.client, tc.typer)
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

func TestUnbind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"SuccessfulDelete": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
				MockDelete: test.NewMockDeleteFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"DeleteError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
				MockDelete: test.NewMockDeleteFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIBinder(tc.client, tc.typer)
			err := api.Unbind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Unbind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Unbind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
func TestStatusUnbind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"SuccessfulDelete": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				MockDelete:       test.NewMockDeleteFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
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
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"DeleteError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				MockDelete:       test.NewMockDeleteFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIStatusBinder(tc.client, tc.typer)
			err := api.Unbind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Unbind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Unbind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClaimRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		cm  resource.Claim
	}

	type want struct {
		err error
		cm  resource.Claim
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
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIClaimFinalizer(tc.client, finalizer)
			err := api.RemoveFinalizer(tc.args.ctx, tc.args.cm)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Claim: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIClaimFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		cm  resource.Claim
	}

	type want struct {
		err error
		cm  resource.Claim
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
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIClaimFinalizer(tc.client, finalizer)
			err := api.AddFinalizer(tc.args.ctx, tc.args.cm)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Claim: -want, +got:\n%s", diff)
			}
		})
	}
}
