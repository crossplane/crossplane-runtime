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

package claimdefaulting

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &Reconciler{}

func TestReconciler(t *testing.T) {
	name := "coolName"
	uid := types.UID("definitely-a-uuid")

	type args struct {
		m  manager.Manager
		of resource.ClaimKind
		to resource.ClassKind
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"ClaimNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"ClaimHasClassRef": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							c := o.(*fake.Claim)
							*c = fake.Claim{ClassReferencer: fake.ClassReferencer{Ref: &corev1.ObjectReference{}}}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ListClassesError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:  test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{err: errors.Wrap(errBoom, errListClasses)},
		},
		"NoClassesAnnotatedDefault": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							u := &unstructured.Unstructured{}
							u.SetGroupVersionKind(fake.GVK(&fake.Class{}))
							u.SetName(name)
							u.SetUID(uid)
							l := o.(*unstructured.UnstructuredList)
							l.Items = []unstructured.Unstructured{*u}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{result: reconcile.Result{RequeueAfter: aShortWait}},
		},
		"UpdateClaimError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							u := &unstructured.Unstructured{}
							u.SetGroupVersionKind(fake.GVK(&fake.Class{}))
							u.SetName(name)
							u.SetUID(uid)
							u.SetAnnotations(map[string]string{v1alpha1.AnnotationDefaultClassKey: v1alpha1.AnnotationDefaultClassValue})
							l := o.(*unstructured.UnstructuredList)
							l.Items = []unstructured.Unstructured{*u}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{err: errors.Wrap(errBoom, errUpdateClaim)},
		},
		"Successful": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							u := &unstructured.Unstructured{}
							u.SetGroupVersionKind(fake.GVK(&fake.Class{}))
							u.SetName(name)
							u.SetUID(uid)
							u.SetAnnotations(map[string]string{v1alpha1.AnnotationDefaultClassKey: v1alpha1.AnnotationDefaultClassValue})
							l := o.(*unstructured.UnstructuredList)
							l.Items = []unstructured.Unstructured{*u}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &fake.Claim{}
							want.SetClassReference(&corev1.ObjectReference{
								APIVersion: fake.GVK(&fake.Class{}).GroupVersion().String(),
								Kind:       fake.GVK(&fake.Class{}).Kind,
								Name:       name,
								UID:        uid,
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.Claim{}),
				},
				of: resource.ClaimKind(fake.GVK(&fake.Claim{})),
				to: resource.ClassKind(fake.GVK(&fake.Class{})),
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.of, tc.args.to, WithDefaultingJitterer(func() {}))
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

func TestReconcilerRandomness(t *testing.T) {
	classes := 10
	reconciles := 100
	refs := make([]*corev1.ObjectReference, 0)

	newClass := func(i int) unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetUID(types.UID(strconv.Itoa(i)))
		u.SetAnnotations(map[string]string{v1alpha1.AnnotationDefaultClassKey: v1alpha1.AnnotationDefaultClassValue})
		return *u
	}

	m := &fake.Manager{
		Client: &test.MockClient{
			MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
				c := o.(*fake.Claim)
				*c = fake.Claim{ClassSelector: fake.ClassSelector{Sel: &metav1.LabelSelector{}}}
				return nil
			}),
			MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
				l := o.(*unstructured.UnstructuredList)
				for i := 0; i < classes; i++ {
					l.Items = append(l.Items, newClass(i))
				}
				return nil
			}),
			MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				ls := obj.(resource.ClassReferencer)
				refs = append(refs, ls.GetClassReference())
				return nil
			}),
		},
		Scheme: fake.SchemeWith(&fake.Claim{}),
	}

	r := NewReconciler(m,
		resource.ClaimKind(fake.GVK(&fake.Claim{})),
		resource.ClassKind(fake.GVK(&fake.Class{})),
		WithDefaultingJitterer(func() {}))

	for i := 0; i < reconciles; i++ {
		r.Reconcile(reconcile.Request{})
	}

	distribution := map[types.UID]int{}
	for _, ref := range refs {
		distribution[ref.UID]++
	}

	// The goal here is to test whether we're random-ish, i.e. that we're not
	// picking the same class every time.
	if len(distribution) < 2 {
		t.Errorf("want > 1 resource classes selected, got %d", len(distribution))
	}
}
