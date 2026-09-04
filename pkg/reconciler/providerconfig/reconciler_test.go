/*
Copyright 2020 The Crossplane Authors.

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

package providerconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

// This can't live in fake, because it would cause an import cycle due to
// GetItems returning managed.ProviderConfigUsage.
type ProviderConfigUsageList struct {
	client.ObjectList
	Items []resource.ProviderConfigUsage
}

func (p *ProviderConfigUsageList) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (p *ProviderConfigUsageList) DeepCopyObject() runtime.Object {
	out := &ProviderConfigUsageList{}

	j, err := json.Marshal(p) //nolint:musttag // We're just using this to round-trip convert.
	if err != nil {
		panic(err)
	}

	_ = json.Unmarshal(j, out) //nolint:musttag // We're just using this to round-trip convert.

	return out
}

func (p *ProviderConfigUsageList) GetItems() []resource.ProviderConfigUsage {
	return p.Items
}

func TestReconciler(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	uid := types.UID("so-unique")
	ctrl := true

	type args struct {
		m  manager.Manager
		of resource.ProviderConfigKinds
	}

	type want struct {
		result  reconcile.Result
		err     error
		request reconcile.Request
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetProviderConfigError": {
			reason: "Errors getting a provider config should be returned",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    errors.Wrap(errBoom, errGetPC),
			},
		},
		"ProviderConfigNotFound": {
			reason: "We should return without requeueing if the provider config no longer exists",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"ListProviderConfigUsageError": {
			reason: "We should requeue after a short wait if we encounter an error listing provider config usages",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:  test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"DeleteProviderConfigUsageError": {
			reason: "We should requeue after a short wait if we encounter an error deleting a provider config usage",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*ProviderConfigUsageList)
							l.Items = []resource.ProviderConfigUsage{
								&fake.ProviderConfigUsage{},
							}

							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"BlockDeleteWhileInUse": {
			reason: "We should return without requeueing if the provider config is still in use",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)
							pc.SetUID(uid)

							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*ProviderConfigUsageList)
							l.Items = []resource.ProviderConfigUsage{
								&fake.ProviderConfigUsage{
									ObjectMeta: metav1.ObjectMeta{
										OwnerReferences: []metav1.OwnerReference{{
											UID:        uid,
											Controller: &ctrl,
										}},
									},
								},
							}

							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should requeue after a short wait if we encounter an error while removing our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)

							return nil
						}),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulDelete": {
			reason: "We should return without requeueing when we successfully remove our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)

							return nil
						}),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"AddFinalizerError": {
			reason: "We should requeue after a short wait if we encounter an error while adding our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"UpdateStatusError": {
			reason: "We return errors encountered while updating our status",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"SuccessfulSetUsers": {
			reason: "We should return without requeuing if we successfully update our user count",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"ListUsagesScopedToNamespaceWhenNamespaced": {
			reason: "When ProviderConfig is namespaced, List should be called with InNamespace option",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetNamespace("test-ns")
							pc.SetName("my-pc")
							return nil
						}),
						MockList: func(_ context.Context, _ client.ObjectList, opts ...client.ListOption) error {
							// Capture and verify list options: should include InNamespace("test-ns")
							listOpts := &client.ListOptions{}
							for _, opt := range opts {
								opt.ApplyToList(listOpts)
							}
							if listOpts.Namespace != "test-ns" {
								t.Errorf("List called with namespace %q, want InNamespace(\"test-ns\")", listOpts.Namespace)
							}
							return nil
						},
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result:  reconcile.Result{Requeue: false},
				request: reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-pc", Namespace: "test-ns"}},
			},
		},
		"ListUsagesNotScopedToNamespaceWhenClusterScoped": {
			reason: "When ProviderConfig is cluster-scoped (empty namespace), List should not include InNamespace",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetNamespace("") // cluster-scoped
							pc.SetName("my-pc")
							return nil
						}),
						MockList: func(_ context.Context, _ client.ObjectList, opts ...client.ListOption) error {
							listOpts := &client.ListOptions{}
							for _, opt := range opts {
								opt.ApplyToList(listOpts)
							}
							if listOpts.Namespace != "" {
								t.Errorf("List called with namespace %q for cluster-scoped ProviderConfig, want empty", listOpts.Namespace)
							}
							return nil
						},
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result:  reconcile.Result{Requeue: false},
				request: reconcile.Request{NamespacedName: types.NamespacedName{Name: "my-pc"}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.of)

			req := tc.want.request
			got, err := r.Reconcile(context.Background(), req)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestReapOrphanedProviderConfigUsage ensures a usage is released after its
// owner is gone.
func TestReapOrphanedProviderConfigUsage(t *testing.T) {
	now := metav1.Now()
	uid := types.UID("owner-uid")
	ctrl := true
	reaped := false

	pcu := &fake.ProviderConfigUsage{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pcu",
			DeletionTimestamp: &now,
			Finalizers:        []string{resource.ProviderConfigUsageFinalizer},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "example.org/v1",
				Kind:       "Thing",
				Name:       "thing",
				UID:        uid,
				Controller: &ctrl,
			}},
		},
	}

	c := &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			// Return NotFound only for the usage owner.
			if _, ok := obj.(*unstructured.Unstructured); ok {
				return kerrors.NewNotFound(schema.GroupResource{Group: "example.org", Resource: "things"}, "thing")
			}
			return nil
		}),
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			// Test-only; always our list type.
			obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{pcu}
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
			// Record an update that removes the usage finalizer.
			if u, ok := obj.(*fake.ProviderConfigUsage); ok && len(u.GetFinalizers()) == 0 {
				reaped = true
			}
			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
	}

	m := &fake.Manager{
		Client: c,
		Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
	}
	r := NewReconciler(m, resource.ProviderConfigKinds{
		Config:    fake.GVK(&fake.ProviderConfig{}),
		Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
		UsageList: fake.GVK(&ProviderConfigUsageList{}),
	})

	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}}); err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %v", err)
	}
	if !reaped {
		t.Error("Reconcile(...): expected the orphaned ProviderConfigUsage finalizer to be released, but it was not")
	}
}

// TestReapProviderConfigUsageWithoutController ensures a usage that lost its
// controller reference has its finalizer released before it's deleted. Nothing
// else can release it, so deleting it while finalized would leave it
// terminating forever.
func TestReapProviderConfigUsageWithoutController(t *testing.T) {
	released := false
	deleted := false

	pcu := &fake.ProviderConfigUsage{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "pcu",
			Finalizers: []string{resource.ProviderConfigUsageFinalizer},
		},
	}

	c := &test.MockClient{
		MockGet: test.NewMockGetFn(nil),
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			// Test-only; always our list type.
			obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{pcu}
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
			if u, ok := obj.(*fake.ProviderConfigUsage); ok && len(u.GetFinalizers()) == 0 {
				released = true
			}
			return nil
		}),
		MockDelete: test.NewMockDeleteFn(nil, func(_ client.Object) error {
			if !released {
				t.Error("Reconcile(...): the ProviderConfigUsage was deleted before its finalizer was released")
			}

			deleted = true

			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
	}

	m := &fake.Manager{
		Client: c,
		Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
	}
	r := NewReconciler(m, resource.ProviderConfigKinds{
		Config:    fake.GVK(&fake.ProviderConfig{}),
		Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
		UsageList: fake.GVK(&ProviderConfigUsageList{}),
	})

	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}}); err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %v", err)
	}

	if !released {
		t.Error("Reconcile(...): expected the ProviderConfigUsage finalizer to be released, but it was not")
	}

	if !deleted {
		t.Error("Reconcile(...): expected the ProviderConfigUsage to be deleted, but it was not")
	}
}

// TestReapProviderConfigUsageWhoseOwnerFinishedTeardown ensures a usage is
// released after its deleting owner removes its controller finalizers.
func TestReapProviderConfigUsageWhoseOwnerFinishedTeardown(t *testing.T) {
	now := metav1.Now()
	uid := types.UID("owner-uid")
	ctrl := true

	cases := map[string]struct {
		reason          string
		ownerDeleting   bool
		ownerFinalizers []string
		wantReaped      bool
		wantRequeue     bool
	}{
		"OwnerFinishedTeardown": {
			reason:          "A deleting owner holding only Kubernetes' propagation finalizer has completed teardown, so the usage must be released.",
			ownerDeleting:   true,
			ownerFinalizers: []string{"foregroundDeletion"},
			wantReaped:      true,
			wantRequeue:     false,
		},
		"OwnerHoldsNoFinalizersAtAll": {
			reason:          "A deleting owner with no finalizers left has nothing outstanding, so the usage must be released.",
			ownerDeleting:   true,
			ownerFinalizers: nil,
			wantReaped:      true,
			wantRequeue:     false,
		},
		"OwnerStillHoldsItsOwnFinalizer": {
			reason:          "A deleting owner that still holds its own finalizer has not finished tearing down. The usage must be left alone, and because we watch usages and ProviderConfigs - not managed resources - nothing will notify us when that owner finishes, so we must ask to be re-queued.",
			ownerDeleting:   true,
			ownerFinalizers: []string{"finalizer.managedresource.crossplane.io", "foregroundDeletion"},
			wantReaped:      false,
			wantRequeue:     true,
		},
		"OwnerIsNotDeleting": {
			reason:          "A live owner still needs its ProviderConfig, so the usage must be left alone - and re-checked, since nothing will tell us when it starts deleting.",
			ownerDeleting:   false,
			ownerFinalizers: nil,
			wantReaped:      false,
			wantRequeue:     true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			reaped := false
			pcu := &fake.ProviderConfigUsage{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pcu",
					DeletionTimestamp: &now,
					Finalizers:        []string{resource.ProviderConfigUsageFinalizer},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "example.org/v1",
						Kind:       "Thing",
						Name:       "thing",
						UID:        uid,
						Controller: &ctrl,
					}},
				},
			}

			c := &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
					// Return the current usage owner.
					u, ok := obj.(*unstructured.Unstructured)
					if !ok {
						return nil
					}
					u.SetName("thing")
					u.SetUID(uid)
					u.SetFinalizers(tc.ownerFinalizers)
					if tc.ownerDeleting {
						u.SetDeletionTimestamp(&now)
					}
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					// Test-only; always our list type.
					obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{pcu}
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
					if u, ok := obj.(*fake.ProviderConfigUsage); ok && len(u.GetFinalizers()) == 0 {
						reaped = true
					}
					return nil
				}),
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
			}

			m := &fake.Manager{
				Client: c,
				Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
			}
			r := NewReconciler(m, resource.ProviderConfigKinds{
				Config:    fake.GVK(&fake.ProviderConfig{}),
				Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
				UsageList: fake.GVK(&ProviderConfigUsageList{}),
			})

			got, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}})
			if err != nil {
				t.Fatalf("%s\nReconcile(...): unexpected error: %v", tc.reason, err)
			}
			if diff := cmp.Diff(tc.wantReaped, reaped); diff != "" {
				t.Errorf("%s\nReconcile(...): -want usage released, +got:\n%s", tc.reason, diff)
			}
			// The reconciler must requeue because it does not watch the owner.
			if diff := cmp.Diff(tc.wantRequeue, got.RequeueAfter > 0); diff != "" {
				t.Errorf("%s\nReconcile(...): -want a re-queue, +got (result %+v):\n%s", tc.reason, got, diff)
			}
		})
	}
}

// A recorder that captures the events it is asked to record.
type recorder struct{ events []event.Event }

func (r *recorder) Event(_ runtime.Object, e event.Event) { r.events = append(r.events, e) }

func (r *recorder) WithAnnotations(_ ...string) event.Recorder { return r }

// terminatingUsage returns a usage that is being deleted, and that is waiting
// for its owner to release it.
func terminatingUsage(name, owner string, uid types.UID, deleted *metav1.Time) *fake.ProviderConfigUsage {
	ctrl := true

	return &fake.ProviderConfigUsage{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			DeletionTimestamp: deleted,
			Finalizers:        []string{resource.ProviderConfigUsageFinalizer},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "example.org/v1",
				Kind:       "Thing",
				Name:       owner,
				UID:        uid,
				Controller: &ctrl,
			}},
		},
	}
}

// TestReapUsageWhileProviderConfigLives ensures a terminating usage does not
// remain stuck when its owner is gone, even if its ProviderConfig is not being
// deleted.
func TestReapUsageWhileProviderConfigLives(t *testing.T) {
	now := metav1.Now()
	pcu := terminatingUsage("pcu", "thing", types.UID("owner-uid"), &now)

	var users int64
	released := false

	c := &test.MockClient{
		MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
			if _, ok := obj.(*unstructured.Unstructured); ok {
				return kerrors.NewNotFound(schema.GroupResource{Group: "example.org", Resource: "things"}, key.Name)
			}
			return nil
		},
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			// Test-only; always our list type.
			obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{pcu}
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
			if u, ok := obj.(*fake.ProviderConfigUsage); ok && len(u.GetFinalizers()) == 0 {
				released = true
			}
			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
			if pc, ok := obj.(*fake.ProviderConfig); ok {
				users = pc.GetUsers()
			}
			return nil
		}),
	}

	m := &fake.Manager{
		Client: c,
		Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
	}
	r := NewReconciler(m, resource.ProviderConfigKinds{
		Config:    fake.GVK(&fake.ProviderConfig{}),
		Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
		UsageList: fake.GVK(&ProviderConfigUsageList{}),
	})

	got, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}})
	if err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %v", err)
	}

	if diff := cmp.Diff(reconcile.Result{Requeue: false}, got); diff != "" {
		t.Errorf("Reconcile(...): -want, +got:\n%s", diff)
	}

	if !released {
		t.Error("Reconcile(...): did not release the terminating usage")
	}

	if diff := cmp.Diff(int64(0), users); diff != "" {
		t.Errorf("Reconcile(...): -want users, +got users:\n%s", diff)
	}
}

// TestBlockDeletionWhenUsageOwnerIsUnreadable ensures we keep blocking
// ProviderConfig deletion - loudly - when we can't tell whether a usage's owner
// still needs it. Releasing the usage would let the ProviderConfig and its
// credentials go while the owner may still be deleting its external resource.
func TestBlockDeletionWhenUsageOwnerIsUnreadable(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	pcu := terminatingUsage("pcu", "thing", types.UID("owner-uid"), &now)

	var users int64

	c := &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			if pc, ok := obj.(*fake.ProviderConfig); ok {
				pc.SetDeletionTimestamp(&now)
				pc.SetFinalizers([]string{finalizer})
				return nil
			}
			// The owner's kind may no longer be served, for example.
			if _, ok := obj.(*unstructured.Unstructured); ok {
				return errBoom
			}
			return nil
		}),
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			// Test-only; always our list type.
			obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{pcu}
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
			// Neither the usage nor the ProviderConfig may be released.
			t.Errorf("Reconcile(...): unexpected update of %T %s", obj, obj.GetName())
			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
			if pc, ok := obj.(*fake.ProviderConfig); ok {
				users = pc.GetUsers()
			}
			return nil
		}),
	}

	m := &fake.Manager{
		Client: c,
		Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
	}
	rec := &recorder{}
	r := NewReconciler(m, resource.ProviderConfigKinds{
		Config:    fake.GVK(&fake.ProviderConfig{}),
		Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
		UsageList: fake.GVK(&ProviderConfigUsageList{}),
	}, WithRecorder(rec))

	got, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}})
	if err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %v", err)
	}

	if diff := cmp.Diff(reconcile.Result{RequeueAfter: shortWait}, got); diff != "" {
		t.Errorf("Reconcile(...): -want, +got:\n%s", diff)
	}

	// The usage must still be counted, and our status must say so - a silent
	// requeue loop would leave nothing to debug.
	if diff := cmp.Diff(int64(1), users); diff != "" {
		t.Errorf("Reconcile(...): -want users, +got users:\n%s", diff)
	}

	want := []event.Event{
		event.Warning(reasonAccount, errors.Wrap(errBoom, errGetPCUOwner)),
		event.Warning(reasonAccount, errors.New("Blocking deletion while usages still exist")),
	}
	if diff := cmp.Diff(want, rec.events); diff != "" {
		t.Errorf("Reconcile(...): -want events, +got events:\n%s", diff)
	}
}

// TestAccountForUsagesWhenOneOwnerIsUnreadable ensures one unreadable owner
// doesn't stop us accounting for - and releasing - the rest of the usages.
func TestAccountForUsagesWhenOneOwnerIsUnreadable(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()

	unreadable := terminatingUsage("pcu-unreadable", "unreadable", types.UID("unreadable-uid"), &now)
	orphaned := terminatingUsage("pcu-orphaned", "orphaned", types.UID("orphaned-uid"), &now)

	var users int64

	released := []string{}

	c := &test.MockClient{
		MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
			if pc, ok := obj.(*fake.ProviderConfig); ok {
				pc.SetDeletionTimestamp(&now)
				pc.SetFinalizers([]string{finalizer})
				return nil
			}
			if _, ok := obj.(*unstructured.Unstructured); !ok {
				return nil
			}
			if key.Name == "unreadable" {
				return errBoom
			}
			return kerrors.NewNotFound(schema.GroupResource{Group: "example.org", Resource: "things"}, key.Name)
		},
		MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
			// Test-only; always our list type.
			obj.(*ProviderConfigUsageList).Items = []resource.ProviderConfigUsage{unreadable, orphaned}
			return nil
		}),
		MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
			if u, ok := obj.(*fake.ProviderConfigUsage); ok && len(u.GetFinalizers()) == 0 {
				released = append(released, u.GetName())
			}
			return nil
		}),
		MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
			if pc, ok := obj.(*fake.ProviderConfig); ok {
				users = pc.GetUsers()
			}
			return nil
		}),
	}

	m := &fake.Manager{
		Client: c,
		Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &fake.ProviderConfigUsage{}, &ProviderConfigUsageList{}),
	}
	r := NewReconciler(m, resource.ProviderConfigKinds{
		Config:    fake.GVK(&fake.ProviderConfig{}),
		Usage:     fake.GVK(&fake.ProviderConfigUsage{}),
		UsageList: fake.GVK(&ProviderConfigUsageList{}),
	})

	got, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "pc"}})
	if err != nil {
		t.Fatalf("Reconcile(...): unexpected error: %v", err)
	}

	if diff := cmp.Diff(reconcile.Result{RequeueAfter: shortWait}, got); diff != "" {
		t.Errorf("Reconcile(...): -want, +got:\n%s", diff)
	}

	if diff := cmp.Diff([]string{"pcu-orphaned"}, released); diff != "" {
		t.Errorf("Reconcile(...): -want released usages, +got:\n%s", diff)
	}

	// Only the usage we couldn't account for still blocks deletion.
	if diff := cmp.Diff(int64(1), users); diff != "" {
		t.Errorf("Reconcile(...): -want users, +got users:\n%s", diff)
	}
}
