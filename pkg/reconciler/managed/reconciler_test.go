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
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &Reconciler{}

func TestReconciler(t *testing.T) {
	type args struct {
		m  manager.Manager
		mg resource.ManagedKind
		o  []ReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	now := metav1.Now()

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetManagedError": {
			reason: "Any error (except not found) encountered while getting the resource under reconciliation should be returned.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetManaged)},
		},
		"ManagedNotFound": {
			reason: "Not found errors encountered while getting the resource under reconciliation should be ignored.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"UnpublishConnectionDetailsDeletionPolicyDeleteOrpahn": {
			reason: "Errors unpublishing connection details should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionOrphan)
							want.SetConditions(xpv1.Deleting())
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors unpublishing connection details should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithConnectionPublishers(ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.Managed, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"RemoveFinalizerErrorDeletionPolicyOrphan": {
			reason: "Errors removing the managed resource finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionOrphan)
							want.SetConditions(xpv1.Deleting())
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors removing the managed resource finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"DeleteSuccessfulDeletionPolicyOrphan": {
			reason: "Successful managed resource deletion with deletion policy Orphan should not trigger a requeue or status update.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"InitializeError": {
			reason: "Errors initializing the managed resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors initializing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(InitializerFn(func(_ context.Context, mg resource.Managed) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalCreatePending": {
			reason: "We should return early if the managed resource appears to be pending creation. We might have leaked a resource and don't want to create another.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							meta.SetExternalCreatePending(obj, now.Time)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, now.Time)
							want.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.New(errCreateIncomplete)))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "We should update our status when we're asked to reconcile a managed resource that is pending creation."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(InitializerFn(func(_ context.Context, mg resource.Managed) error { return nil })),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ResolveReferencesError": {
			reason: "Errors during reference resolution references should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors during reference resolution should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, res resource.Managed) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalConnectError": {
			reason: "Errors connecting to the provider should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, got client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileConnect)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								reason := "Errors connecting to the provider should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDisconnectError": {
			reason: "Error disconnecting from the provider should not trigger requeue.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful no-op reconcile should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnectDisconnecter(NewExternalConnectDisconnecter(
						ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
							c := &ExternalClientFns{
								ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
									return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
								},
							}
							return c, nil
						}), ExternalDisconnectorFn(func(_ context.Context) error {
							return errBoom
						})),
					),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultpollInterval}},
		},
		"ExternalObserveErrorDisconnectError": {
			reason: "Errors disconnecting from the provider after error observing the external resource should trigger a requeue after a short wait and return error.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileObserve)))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors observing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithExternalConnectDisconnecter(NewExternalConnectDisconnecter(
						ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
							c := &ExternalClientFns{
								ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
									return ExternalObservation{}, errBoom
								},
							}
							return c, nil
						}), ExternalDisconnectorFn(func(_ context.Context) error {
							return errBoom
						})),
					),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalObserveError": {
			reason: "Errors observing the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileObserve)))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors observing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{}, errBoom
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreationGracePeriod": {
			reason: "If our resource appears not to exist during the creation grace period we should return early.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							meta.SetExternalCreateSucceeded(obj, time.Now())
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithCreationGracePeriod(1 * time.Minute),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDeleteError": {
			reason: "Errors deleting the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionDelete)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileDelete)))
							want.SetConditions(xpv1.Deleting())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "An error deleting an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) error {
								return errBoom
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDeleteSuccessful": {
			reason: "A deleted managed resource with the 'delete' reclaim policy should delete its external resource then requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionDelete)
							want.SetConditions(xpv1.ReconcileSuccess())
							want.SetConditions(xpv1.Deleting())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A deleted external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) error {
								return nil
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UnpublishConnectionDetailsDeletionPolicyDeleteError": {
			reason: "Errors unpublishing connection details should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionDelete)
							want.SetConditions(xpv1.Deleting())
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors unpublishing connection details should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.Managed, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"RemoveFinalizerErrorDeletionPolicyDelete": {
			reason: "Errors removing the managed resource finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionDelete)
							want.SetConditions(xpv1.Deleting())
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors removing the managed resource finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"DeleteSuccessfulDeletionPolicyDelete": {
			reason: "Successful managed resource deletion with deletion policy Delete should not trigger a requeue or status update.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := obj.(*fake.Managed)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PublishObservationConnectionDetailsError": {
			reason: "Errors publishing connection details after observation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after observation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.Managed, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"AddFinalizerError": {
			reason: "Errors adding a finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors adding a finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateCreatePendingError": {
			reason: "Errors while updating our external-create-pending annotation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, time.Now())
							want.SetConditions(xpv1.Creating(), xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManaged)))
							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors while creating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, errBoom
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreateExternalError": {
			reason: "Errors while creating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateFailed(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileCreate)))
							want.SetConditions(xpv1.Creating())
							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors while creating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, errBoom
							},
						}
						return c, nil
					})),
					// We simulate our critical annotation update failing too here.
					// This is mostly just to exercise the code, which just creates a log and an event.
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(ctx context.Context, o client.Object) error { return errBoom })),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateCriticalAnnotationsError": {
			reason: "Errors updating critical annotations after creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManagedAnnotations)))
							want.SetConditions(xpv1.Creating())
							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors updating critical annotations after creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, nil
							},
						}
						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(ctx context.Context, o client.Object) error { return errBoom })),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"PublishCreationConnectionDetailsError": {
			reason: "Errors publishing connection details after creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errBoom))
							want.SetConditions(xpv1.Creating())
							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors publishing connection details after creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								cd := ConnectionDetails{"create": []byte{}}
								return ExternalCreation{ConnectionDetails: cd}, nil
							},
						}
						return c, nil
					})),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(ctx context.Context, o client.Object) error { return nil })),
					WithConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.Managed, cd ConnectionDetails) error {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after a creation.
							if _, ok := cd["create"]; ok {
								return errBoom
							}
							return nil
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreateSuccessful": {
			reason: "Successful managed resource creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileSuccess())
							want.SetConditions(xpv1.Creating())
							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(ctx context.Context, o client.Object) error { return nil })),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"LateInitializeUpdateError": {
			reason: "Errors updating a managed resource to persist late initialized fields should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManaged)))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors updating a managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalResourceUpToDate": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful no-op reconcile should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultpollInterval}},
		},
		"UpdateExternalError": {
			reason: "Errors while updating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileUpdate)))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors while updating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, errBoom
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"PublishUpdateConnectionDetailsError": {
			reason: "Errors publishing connection details after an update should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after an update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								cd := ConnectionDetails{"update": []byte{}}
								return ExternalUpdate{ConnectionDetails: cd}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.Managed, cd ConnectionDetails) error {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after an update.
							if _, ok := cd["update"]; ok {
								return errBoom
							}
							return nil
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateSuccessful": {
			reason: "A successful managed resource update should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
							want := &fake.Managed{}
							want.SetConditions(xpv1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Managed{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.Managed{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
						}
						return c, nil
					})),
					WithConnectionPublishers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultpollInterval}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.mg, tc.args.o...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
