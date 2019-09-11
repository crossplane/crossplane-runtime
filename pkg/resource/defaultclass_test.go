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
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var _ reconcile.Reconciler = &DefaultClassReconciler{}

type MockObjectConvertor struct {
	runtime.ObjectConvertor
}

func (m *MockObjectConvertor) Convert(in, out, context interface{}) error {
	i, inok := in.(*unstructured.Unstructured)
	if !inok {
		return errors.Errorf("expected conversion input to be of type %s", reflect.TypeOf(unstructured.Unstructured{}).String())
	}
	_, outok := out.(*MockPortableClass)
	if !outok {
		return errors.Errorf("expected conversion input to be of type %s", reflect.TypeOf(MockPortableClass{}).String())
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(i.Object, out); err != nil {
		return err
	}
	return nil
}

func TestDefaultClassReconcile(t *testing.T) {
	type args struct {
		m  manager.Manager
		of ClaimKind
		by PortableClassKind
		o  []DefaultClassReconcilerOption
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")

	classRef := &corev1.ObjectReference{
		Name:      "default-class",
		Namespace: "default-namespace",
	}
	portable := MockPortableClass{}
	portable.SetNonPortableClassReference(classRef)
	portableList := []PortableClass{
		&MockPortableClass{},
	}
	portableListTooMany := []PortableClass{
		&MockPortableClass{},
		&MockPortableClass{},
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"GetClaimError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockPortableClassList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PortableClassKind{Singular: MockGVK(&MockPortableClass{}), Plural: MockGVK(&MockPortableClassList{})},
			},
			want: want{err: errors.Wrap(errBoom, errGetClaim)},
		},
		"ListPoliciesError": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(errBoom),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errFailedList)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockPortableClassList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PortableClassKind{Singular: MockGVK(&MockPortableClass{}), Plural: MockGVK(&MockPortableClassList{})},
			},
			want: want{result: reconcile.Result{}},
		},
		"NoDefaultClass": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPortableClassList:
								*o = MockPortableClassList{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errNoPortableClass)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockPortableClassList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PortableClassKind{Singular: MockGVK(&MockPortableClass{}), Plural: MockGVK(&MockPortableClassList{})},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"MultipleDefaultClasses": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPortableClassList:
								pl := &MockPortableClassList{}
								pl.SetPortableClassItems(portableListTooMany)
								*o = *pl
								return nil
							default:
								return errUnexpected
							}
						}),
						MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetConditions(v1alpha1.ReconcileError(errors.New(errMultiplePortableClasses)))
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockPortableClassList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PortableClassKind{Singular: MockGVK(&MockPortableClass{}), Plural: MockGVK(&MockPortableClassList{})},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultClassWait}},
		},
		"Successful": {
			args: args{
				m: &MockManager{
					c: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockClaim:
								*o = MockClaim{}
								return nil
							default:
								return errUnexpected
							}
						}),
						MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
							switch o := o.(type) {
							case *MockPortableClassList:
								pl := &MockPortableClassList{}
								pl.SetPortableClassItems(portableList)
								*o = *pl
								return nil
							default:
								return errUnexpected
							}
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &MockClaim{}
							want.SetPortableClassReference(&corev1.LocalObjectReference{Name: portable.GetName()})
							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockPortableClassList{}),
				},
				of: ClaimKind(MockGVK(&MockClaim{})),
				by: PortableClassKind{Singular: MockGVK(&MockPortableClass{}), Plural: MockGVK(&MockPortableClassList{})},
				o:  []DefaultClassReconcilerOption{WithObjectConverter(&MockObjectConvertor{})},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewDefaultClassReconciler(tc.args.m, tc.args.of, tc.args.by, tc.args.o...)
			fmt.Println(name)
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
