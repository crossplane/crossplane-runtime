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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &ClaimReconciler{}

func TestClaimReconciler(t *testing.T) {
	type args struct {
		m    manager.Manager
		of   ClaimKind
		use  ClassKind
		with ManagedKind
		o    []ClaimReconcilerOption
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
				m: &fake.MockManager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"GetManagedError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ManagedNotFound": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"UnbindError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ Claim, _ Managed) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"UnbindSuccess": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ Claim, _ Managed) error { return nil }}),
					WithClaimFinalizer(ClaimFinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ Claim) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"RemoveClaimFinalizerError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ Claim, _ Managed) error { return nil }}),
					WithClaimFinalizer(ClaimFinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ Claim) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"SuccessfulDelete": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetDeletionTimestamp(&now)
								*o = *cm
								return nil
							default:
								return errUnexpected
							}
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithBinder(BinderFns{UnbindFn: func(_ context.Context, _ Claim, _ Managed) error { return nil }}),
					WithClaimFinalizer(ClaimFinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ Claim) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ClassReferenceNotSet": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								*o = fake.MockClaim{}
								return nil
							case *fake.MockManaged:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"GetResourceClassError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockClass:
								return errBoom
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ConfigureManagedError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockClass:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{WithManagedConfigurators(ManagedConfiguratorFn(
					func(_ context.Context, _ Claim, _ Class, _ Managed) error { return errBoom },
				))},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"CreateManagedError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetClassReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockClass:
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetClassReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConfigurators(ManagedConfiguratorFn(
						func(_ context.Context, _ Claim, _ Class, _ Managed) error { return nil },
					)),
					WithClaimFinalizer(ClaimFinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ Claim) error { return nil }},
					),
					WithManagedCreator(ManagedCreatorFn(
						func(_ context.Context, _ Claim, _ Class, _ Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"ManagedIsInUnknownBindingPhase": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								// We do not explicitly set a BindingPhase here
								// because the zero value of BindingPhase is
								// BindingPhaseUnset.
								mg := &fake.MockManaged{}
								mg.SetClaimReference(&corev1.ObjectReference{})
								mg.SetCreationTimestamp(now)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ManagedIsInUnbindableBindingPhase": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								mg := &fake.MockManaged{}
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
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PropagateConnectionError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								mg := &fake.MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConnectionPropagator(ManagedConnectionPropagatorFn(
						func(_ context.Context, _ Claim, _ Managed) error { return errBoom },
					)),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"AddFinalizerError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								mg := &fake.MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConnectionPropagator(ManagedConnectionPropagatorFn(
						func(_ context.Context, _ Claim, _ Managed) error { return nil },
					)),
					WithClaimFinalizer(ClaimFinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ Claim) error { return errBoom }},
					),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"BindError": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								mg := &fake.MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(Binding(), v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
				o: []ClaimReconcilerOption{
					WithManagedConnectionPropagator(ManagedConnectionPropagatorFn(
						func(_ context.Context, _ Claim, _ Managed) error { return nil },
					)),
					WithClaimFinalizer(ClaimFinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ Claim) error { return nil }},
					),
					WithBinder(BinderFns{
						BindFn: func(_ context.Context, _ Claim, _ Managed) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"Successful": {
			args: args{
				m: &fake.MockManager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *fake.MockClaim:
								cm := &fake.MockClaim{}
								cm.SetResourceReference(&corev1.ObjectReference{})
								*o = *cm
								return nil
							case *fake.MockManaged:
								mg := &fake.MockManaged{}
								mg.SetCreationTimestamp(now)
								mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.MockClaim{}
							want.SetResourceReference(&corev1.ObjectReference{})
							want.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.MockSchemeWith(&fake.MockClaim{}, &fake.MockClass{}, &fake.MockManaged{}),
				},
				of:   ClaimKind(fake.MockGVK(&fake.MockClaim{})),
				use:  ClassKind(fake.MockGVK(&fake.MockClass{})),
				with: ManagedKind(fake.MockGVK(&fake.MockManaged{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewClaimReconciler(tc.args.m, tc.args.of, tc.args.use, tc.args.with, tc.args.o...)
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
