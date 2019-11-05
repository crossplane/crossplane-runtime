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

	"sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &ManagedReconciler{}

func TestManagedReconciler(t *testing.T) {
	type args struct {
		m  manager.Manager
		mg ManagedKind
		o  []ManagedReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errNotReady := &referencesAccessErr{[]ReferenceStatus{{Name: "cool-res", Status: ReferenceNotReady}}}
	now := metav1.Now()

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetManagedError": {
			reason: "Any error (except not found) encountered while getting the resource under reconciliation should be returned.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetManaged)},
		},
		"ManagedNotFound": {
			reason: "Not found errors encountered while getting the resource under reconciliation should be ignored.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"ExternalConnectError": {
			reason: "Errors connecting to the provider should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, got runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								reason := "Errors connecting to the provider should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"InitializeError": {
			reason: "Errors initializing the managed resource should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors initializing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithExternalConnecter(&NopConnecter{}),
					WithManagedInitializers(ManagedInitializerFn(func(_ context.Context, mg Managed) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ResolveReferencesNotReadyError": {
			reason: "Dependencies on unready references should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionBlocked(errNotReady))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Dependencies on unready references should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithExternalConnecter(&NopConnecter{}),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, res CanReference) error {
						return errNotReady
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ResolveReferencesError": {
			reason: "Errors during reference resolution references should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors during reference resolution should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithExternalConnecter(&NopConnecter{}),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, res CanReference) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalObserveError": {
			reason: "Errors observing the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors observing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{}, errBoom
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalDeleteError": {
			reason: "Errors deleting the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "An error deleting an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ Managed) error {
								return errBoom
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalDeleteSuccessful": {
			reason: "A deleted managed resource with the 'delete' reclaim policy should delete its external resource then requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetReclaimPolicy(v1alpha1.ReclaimDelete)
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A deleted external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ Managed) error {
								return nil
							},
						}
						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"UnpublishConnectionDetailsError": {
			reason: "Errors unpublishing connection details should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors unpublishing connection details should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"RemoveFinalizerError": {
			reason: "Errors removing the managed resource finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetDeletionTimestamp(&now)
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors removing the managed resource finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ Managed) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"DeleteSuccessful": {
			reason: "Successful managed resource deletion should not trigger a requeue or status update.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							mg := obj.(*MockManaged)
							mg.SetDeletionTimestamp(&now)
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PublishObservationConnectionDetailsError": {
			reason: "Errors publishing connection details after observation should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after observation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ Managed, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"AddFinalizerError": {
			reason: "Errors adding a finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors adding a finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"CreateExternalError": {
			reason: "Errors while creating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors while creating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ Managed) (ExternalCreation, error) {
								return ExternalCreation{}, errBoom
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"PublishCreationConnectionDetailsError": {
			reason: "Errors publishing connection details after creation should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ Managed) (ExternalCreation, error) {
								cd := ConnectionDetails{"create": []byte{}}
								return ExternalCreation{ConnectionDetails: cd}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ Managed, cd ConnectionDetails) error {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after a creation.
							if _, ok := cd["create"]; ok {
								return errBoom
							}
							return nil
						},
					}),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"CreateSuccessful": {
			reason: "Successful managed resource creation should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(&NopConnecter{}),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"ExternalResourceUpToDate": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful no-op reconcile should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedLongWait}},
		},
		"UpdateExternalError": {
			reason: "Errors while updating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors while updating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, errBoom
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"PublishUpdateConnectionDetailsError": {
			reason: "Errors publishing connection details after an update should trigger a requeue after a short wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileError(errBoom))
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after an update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
								cd := ConnectionDetails{"update": []byte{}}
								return ExternalUpdate{ConnectionDetails: cd}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(ManagedConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ Managed, cd ConnectionDetails) error {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after an update.
							if _, ok := cd["update"]; ok {
								return errBoom
							}
							return nil
						},
					}),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedShortWait}},
		},
		"UpdateSuccessful": {
			reason: "A successful managed resource update should trigger a requeue after a long wait.",
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.MockStatusUpdateFn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
							want := &MockManaged{}
							want.SetConditions(v1alpha1.ReferenceResolutionSuccess())
							want.SetConditions(v1alpha1.ReconcileSuccess())
							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockManaged{}),
				},
				mg: ManagedKind(MockGVK(&MockManaged{})),
				o: []ManagedReconcilerOption{
					WithManagedInitializers(),
					WithManagedReferenceResolver(ManagedReferenceResolverFn(func(_ context.Context, _ CanReference) error { return nil })),
					WithExternalConnecter(ExternalConnectorFn(func(_ context.Context, mg Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
						}
						return c, nil
					})),
					WithManagedConnectionPublishers(),
					WithManagedFinalizer(ManagedFinalizerFns{AddFinalizerFn: func(_ context.Context, _ Managed) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultManagedLongWait}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagedReconciler(tc.args.m, tc.args.mg, tc.args.o...)
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
