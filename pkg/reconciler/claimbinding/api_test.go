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

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ManagedCreator = &APIManagedCreator{}
	_ Binder         = &APIBinder{}
	_ Binder         = &APIStatusBinder{}
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
	controller := true

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"ControlledError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Controller: &controller}},
				}},
			},
			want: want{
				err: errors.New(errBindControlled),
				cm:  &fake.Claim{},
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Controller: &controller}},
				}},
			},
		},
		"RefMismatchError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
			want: want{
				err: errors.New(errBindMismatch),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
		},
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
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
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
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
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
	controller := true

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"ControlledError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Controller: &controller}},
				}},
			},
			want: want{
				err: errors.New(errBindControlled),
				cm:  &fake.Claim{},
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{Controller: &controller}},
				}},
			},
		},
		"RefMismatchError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
			want: want{
				err: errors.New(errBindMismatch),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
		},
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
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
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
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.AnnotationKeyExternalName: externalName}},
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

	typer := fake.SchemeWith(&fake.Claim{})
	ref := meta.ReferenceTo(&fake.Claim{}, resource.MustGetKind(&fake.Claim{}, typer))

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"RefMismatchError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg:  &fake.Managed{ClaimReferencer: fake.ClaimReferencer{Ref: ref}},
			},
			want: want{
				err: errors.New(errUnbindMismatch),
				mg:  &fake.Managed{ClaimReferencer: fake.ClaimReferencer{Ref: ref}},
			},
		},
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
	typer := fake.SchemeWith(&fake.Claim{})
	ref := meta.ReferenceTo(&fake.Claim{}, resource.MustGetKind(&fake.Claim{}, typer))

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"RefMismatchError": {
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: "I'm different!"}},
				mg:  &fake.Managed{ClaimReferencer: fake.ClaimReferencer{Ref: ref}},
			},
			want: want{
				err: errors.New(errUnbindMismatch),
				mg:  &fake.Managed{ClaimReferencer: fake.ClaimReferencer{Ref: ref}},
			},
		},
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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
			typer: typer,
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: ref},
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

func TestObjectReferenceEqual(t *testing.T) {
	cases := map[string]struct {
		a    *corev1.ObjectReference
		b    *corev1.ObjectReference
		want bool
	}{
		"BothNil": {
			want: true,
		},
		"OneIsNil": {
			a:    &corev1.ObjectReference{},
			want: false,
		},
		"MismatchedAPIVersion": {
			a: &corev1.ObjectReference{
				APIVersion: "v",
			},
			b:    &corev1.ObjectReference{},
			want: false,
		},
		"MismatchedKind": {
			a: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
			},
			b: &corev1.ObjectReference{
				APIVersion: "v",
			},
			want: false,
		},
		"MismatchedNamespace": {
			a: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
				Namespace:  "ns",
			},
			b: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
			},
			want: false,
		},
		"MismatchedName": {
			a: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
				Namespace:  "ns",
				Name:       "cool",
			},
			b: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
				Namespace:  "ns",
			},
			want: false,
		},
		"Match": {
			a: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
				Namespace:  "ns",
				Name:       "cool",
			},
			b: &corev1.ObjectReference{
				APIVersion: "v",
				Kind:       "k",
				Namespace:  "ns",
				Name:       "cool",
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := equal(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("equal(...): want %t, got %t", tc.want, got)
			}
		})
	}
}
