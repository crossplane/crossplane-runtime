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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &Reconciler{}

func TestReconciler(t *testing.T) {
	type args struct {
		m    manager.Manager
		of   resource.ClaimKind
		use  resource.ClassKind
		with resource.ManagedKind
		o    []ReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")
	now := metav1.Now()

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"GetManagedError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ManagedNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"UnbindError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ resource.Claim, _ resource.Managed) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"UnbindSuccess": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ resource.Claim, _ resource.Managed) error { return nil }}),
					WithClaimFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"RemoveClaimFinalizerError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ resource.Claim, _ resource.Managed) error { return nil }}),
					WithClaimFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"SuccessfulDelete": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ resource.Claim, _ resource.Managed) error { return nil }}),
					WithClaimFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ClassReferenceNotSet": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								*o = fake.Claim{}
								return nil
							case *fake.Managed:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"GetResourceClassError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Class:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ConfigureManagedError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Class:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{WithManagedConfigurators(ManagedConfiguratorFn(
					func(_ context.Context, _ resource.Claim, _ resource.Class, _ resource.Managed) error { return errBoom },
				))},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"CreateManagedError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Class:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithManagedConfigurators(ManagedConfiguratorFn(
						func(_ context.Context, _ resource.Claim, _ resource.Class, _ resource.Managed) error { return nil },
					)),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }},
					),
					WithManagedCreator(ManagedCreatorFn(
						func(_ context.Context, _ resource.Claim, _ resource.Class, _ resource.Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ManagedIsInUnknownBindingPhase": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								// We do not explicitly set a BindingPhase here
								// because the zero value of BindingPhase is
								// BindingPhaseUnset.
								mg := &fake.Managed{}
								mg.SetClaimReference(&corev1.ObjectReference{})
								mg.SetCreationTimestamp(now)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ManagedIsInUnbindableBindingPhase": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								mg := &fake.Managed{}
								mg.SetCreationTimestamp(now)
								mg.SetClaimReference(&corev1.ObjectReference{})
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbindable)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PropagateConnectionError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								mg := &fake.Managed{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithManagedConnectionPropagator(resource.ManagedConnectionPropagatorFn(
						func(_ context.Context, _ resource.LocalConnectionSecretOwner, _ resource.Managed) error {
							return errBoom
						},
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"AddFinalizerError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								mg := &fake.Managed{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithManagedConnectionPropagator(resource.ManagedConnectionPropagatorFn(
						func(_ context.Context, _ resource.LocalConnectionSecretOwner, _ resource.Managed) error { return nil },
					)),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }},
					),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"BindError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								mg := &fake.Managed{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithManagedConnectionPropagator(resource.ManagedConnectionPropagatorFn(
						func(_ context.Context, _ resource.LocalConnectionSecretOwner, _ resource.Managed) error { return nil },
					)),
					WithClaimFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }},
					),
					WithBinder(BinderFns{
						BindFn: func(_ context.Context, _ resource.Claim, _ resource.Managed) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"Successful": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Claim:
								cm := &fake.Claim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.Managed:
								mg := &fake.Managed{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
				},
				of:   resource.ClaimKind(fake.GVK(&fake.Claim{})),
				use:  resource.ClassKind(fake.GVK(&fake.Class{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.of, tc.args.use, tc.args.with, tc.args.o...)
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r.Reconcile(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("r.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
