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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

func TestReconciler(t *testing.T) {
	type args struct {
		m    manager.Manager
		of   resource.TargetKind
		with resource.ManagedKind
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	now := metav1.Now()
	ns := "namespace"
	tgname := "cooltarget"
	mgname := "coolmanaged"
	tguid := types.UID("tg-uuid")
	mguid := types.UID("mg-uuid")
	tgcsuid := types.UID("tgcs-uuid")
	mgcsuid := types.UID("mgcs-uuid")
	tgcsname := "cooltargetsecret"
	mgcsname := "coolmanagedsecret"
	mgcsnamespace := "coolns"
	mgcsdata := map[string][]byte{"cool": []byte("data")}
	controller := true

	errBoom := errors.New("boom")
	errUnexpectedSecret := errors.New("unexpected secret name")
	errUnexpected := errors.New("unexpected object type")

	cases := map[string]struct {
		args args
		want want
	}{
		"ErrorGetTarget": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								*o = fake.Target{}
								return errBoom
							default:
								return errUnexpected
							}
						},
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{},
				err:    errors.Wrap(errBoom, errGetTarget),
			},
		},
		"SuccessTargetHasNoSecretRef": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								*o = *tg
								return nil
							default:
								return errUnexpected
							}
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Target{}
							want.SetName(tgname)
							want.SetNamespace(ns)
							want.SetUID(tguid)
							want.SetResourceReference(&corev1.ObjectReference{
								Name: mgname,
							})
							want.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
								Name: string(tguid),
							})
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"TargetWasDeleted": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								tg.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
									Name: tgcsname,
								})
								tg.SetDeletionTimestamp(&now)
								*o = *tg
								return nil
							default:
								return errUnexpected
							}
						},
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"TargetNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								*o = fake.Target{}
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							default:
								return errUnexpected
							}
						},
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"ErrorGetManaged": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								tg.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
									Name: tgcsname,
								})
								*o = *tg
								return nil
							case *fake.Managed:
								*o = fake.Managed{}
								return errBoom
							default:
								return errUnexpected
							}
						},
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Target{}
							want.SetName(tgname)
							want.SetNamespace(ns)
							want.SetUID(tguid)
							want.SetResourceReference(&corev1.ObjectReference{
								Name: mgname,
							})
							want.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: tgcsname})
							want.SetConditions(v1alpha1.SecretPropagationError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"ErrorManagedNotBound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								tg.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
									Name: tgcsname,
								})
								*o = *tg
								return nil
							case *fake.Managed:
								mg := &fake.Managed{ObjectMeta: metav1.ObjectMeta{
									UID:  mguid,
									Name: mgname,
								}}
								mg.SetWriteConnectionSecretToReference(&v1alpha1.SecretReference{
									Name:      mgcsname,
									Namespace: mgcsnamespace,
								})
								mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
								*o = *mg
								return nil
							default:
								return errUnexpected
							}
						},
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Target{}
							want.SetName(tgname)
							want.SetNamespace(ns)
							want.SetUID(tguid)
							want.SetResourceReference(&corev1.ObjectReference{
								Name: mgname,
							})
							want.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: tgcsname})
							want.SetConditions(v1alpha1.SecretPropagationError(errors.New(errManagedResourceIsNotBound)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"ErrorSecretPropagationFailed": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								tg.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
									Name: tgcsname,
								})
								*o = *tg
								return nil
							case *fake.Managed:
								mg := &fake.Managed{ObjectMeta: metav1.ObjectMeta{
									UID:  mguid,
									Name: mgname,
								}}
								mg.SetWriteConnectionSecretToReference(&v1alpha1.SecretReference{
									Name:      mgcsname,
									Namespace: mgcsnamespace,
								})
								mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
								*o = *mg
								return nil
							case *corev1.Secret:
								switch n.Name {
								case tgcsname:
									sc := &corev1.Secret{}
									sc.SetName(tgcsname)
									sc.SetNamespace(ns)
									sc.SetUID(tgcsuid)
									sc.SetOwnerReferences([]metav1.OwnerReference{{
										UID:        tguid,
										Controller: &controller,
									}})
									*o = *sc
									return nil
								case mgcsname:
									sc := &corev1.Secret{}
									sc.SetName(mgcsname)
									sc.SetNamespace(mgcsnamespace)
									sc.SetUID(mgcsuid)
									sc.Data = mgcsdata
									*o = *sc
									return nil
								default:
									return errUnexpectedSecret
								}
							default:
								return errUnexpected
							}
						},
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Target{}
							want.SetName(tgname)
							want.SetNamespace(ns)
							want.SetUID(tguid)
							want.SetResourceReference(&corev1.ObjectReference{
								Name: mgname,
							})
							want.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: tgcsname})
							want.SetConditions(v1alpha1.SecretPropagationError(errors.New(errSecretConflict)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{RequeueAfter: aShortWait},
			},
		},
		"Successful": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch o := o.(type) {
							case *fake.Target:
								tg := &fake.Target{ObjectMeta: metav1.ObjectMeta{
									UID:       tguid,
									Name:      tgname,
									Namespace: ns,
								}}
								tg.SetResourceReference(&corev1.ObjectReference{
									Name: mgname,
								})
								tg.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{
									Name: tgcsname,
								})
								*o = *tg
								return nil
							case *fake.Managed:
								mg := &fake.Managed{ObjectMeta: metav1.ObjectMeta{
									UID:  mguid,
									Name: mgname,
								}}
								mg.SetWriteConnectionSecretToReference(&v1alpha1.SecretReference{
									Name:      mgcsname,
									Namespace: mgcsnamespace,
								})
								mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
								*o = *mg
								return nil
							case *corev1.Secret:
								switch n.Name {
								case tgcsname:
									sc := &corev1.Secret{}
									sc.SetName(tgcsname)
									sc.SetNamespace(ns)
									sc.SetUID(tgcsuid)
									sc.SetOwnerReferences([]metav1.OwnerReference{{
										UID:        tguid,
										Controller: &controller,
									}})
									*o = *sc
									return nil
								case mgcsname:
									sc := &corev1.Secret{}
									sc.SetName(mgcsname)
									sc.SetNamespace(mgcsnamespace)
									sc.SetUID(mgcsuid)
									sc.SetOwnerReferences([]metav1.OwnerReference{{
										UID:        mguid,
										Controller: &controller,
									}})
									sc.Data = mgcsdata
									*o = *sc
									return nil
								default:
									return errUnexpectedSecret
								}
							default:
								return errUnexpected
							}
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &corev1.Secret{}
							want.Data = mgcsdata

							switch got.(metav1.Object).GetName() {
							case tgcsname:
								want.SetName(tgcsname)
								want.SetNamespace(ns)
								want.SetUID(tgcsuid)
								want.SetOwnerReferences([]metav1.OwnerReference{{UID: tguid, Controller: &controller}})
								want.SetAnnotations(map[string]string{
									resource.AnnotationKeyPropagateFromNamespace: mgcsnamespace,
									resource.AnnotationKeyPropagateFromName:      mgcsname,
									resource.AnnotationKeyPropagateFromUID:       string(mgcsuid),
								})
								if diff := cmp.Diff(want, got); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
							case mgcsname:
								want.SetName(mgcsname)
								want.SetNamespace(mgcsnamespace)
								want.SetUID(mgcsuid)
								want.SetOwnerReferences([]metav1.OwnerReference{{UID: mguid, Controller: &controller}})
								want.SetAnnotations(map[string]string{
									strings.Join([]string{resource.AnnotationKeyPropagateToPrefix, string(tgcsuid)}, resource.AnnotationDelimiter): strings.Join([]string{ns, tgcsname}, resource.AnnotationDelimiter),
								})
								if diff := cmp.Diff(want, got); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
							default:
								return errUnexpectedSecret
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Target{}
							want.SetName(tgname)
							want.SetNamespace(ns)
							want.SetUID(tguid)
							want.SetResourceReference(&corev1.ObjectReference{
								Name: mgname,
							})
							want.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: tgcsname})
							want.SetConditions(v1alpha1.SecretPropagationSuccess())
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Target{}, &fake.Managed{}),
				},
				of:   resource.TargetKind(fake.GVK(&fake.Target{})),
				with: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.of, tc.args.with)
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
