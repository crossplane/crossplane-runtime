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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

func TestAnyOf(t *testing.T) {
	cases := map[string]struct {
		fns  []PredicateFn
		obj  runtime.Object
		want bool
	}{
		"PredicatePasses": {
			fns: []PredicateFn{
				func(obj runtime.Object) bool { return false },
				func(obj runtime.Object) bool { return true },
			},
			want: true,
		},
		"NoPredicatesPass": {
			fns: []PredicateFn{
				func(obj runtime.Object) bool { return false },
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := AnyOf(tc.fns...)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("AnyOf(...): -want, +got:\n%s", diff)
			}
		})
	}

}

func TestHasManagedResourceReferenceKind(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		c    client.Client
		s    runtime.ObjectCreater
		kind ManagedKind
		want bool
	}{
		"NotAClassReferencer": {
			c:    &test.MockClient{},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockManaged{}),
			kind: ManagedKind(MockGVK(&MockManaged{})),
			want: false,
		},
		"HasNoResourceReference": {
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockManaged{}),
			obj:  &MockClaim{},
			kind: ManagedKind(MockGVK(&MockManaged{})),
			want: false,
		},
		"HasCorrectResourceReference": {
			s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockManaged{}),
			obj: &MockClaim{
				MockManagedResourceReferencer: MockManagedResourceReferencer{
					Ref: &corev1.ObjectReference{
						APIVersion: MockGVK(&MockManaged{}).GroupVersion().String(),
						Kind:       MockGVK(&MockManaged{}).Kind,
					},
				},
			},
			kind: ManagedKind(MockGVK(&MockManaged{})),
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := HasManagedResourceReferenceKind(tc.kind)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HasManagedResourceReferenceKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestHasDirectClassReferenceKind(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		c    client.Client
		s    runtime.ObjectCreater
		kind NonPortableClassKind
		want bool
	}{
		"NotAClassReferencer": {
			c:    &test.MockClient{},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			kind: NonPortableClassKind(MockGVK(&MockNonPortableClass{})),
			want: false,
		},
		"HasNoDirectClass": {
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockManaged{},
			kind: NonPortableClassKind(MockGVK(&MockNonPortableClass{})),
			want: false,
		},
		"HasCorrectDirectClass": {
			s: MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj: &MockManaged{
				MockNonPortableClassReferencer: MockNonPortableClassReferencer{
					Ref: &corev1.ObjectReference{
						APIVersion: MockGVK(&MockNonPortableClass{}).GroupVersion().String(),
						Kind:       MockGVK(&MockNonPortableClass{}).Kind,
					},
				},
			},
			kind: NonPortableClassKind(MockGVK(&MockNonPortableClass{})),
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := HasDirectClassReferenceKind(tc.kind)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HasDirectClassReferenceKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestHasIndirectClassReferenceKind(t *testing.T) {
	errUnexpected := errors.New("unexpected object type")

	type withoutNamespace struct {
		runtime.Object
		*MockPortableClassReferencer
	}

	cases := map[string]struct {
		obj  runtime.Object
		c    client.Client
		s    runtime.ObjectCreater
		kind ClassKinds
		want bool
	}{
		"NotAPortableClassReferencer": {
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: false,
		},
		"NoPortableClassReference": {
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockClaim{},
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: false,
		},
		"NotANamespacer": {
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  withoutNamespace{MockPortableClassReferencer: &MockPortableClassReferencer{Ref: &corev1.LocalObjectReference{}}},
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: false,
		},
		"GetPortableClassError": {
			c:    &test.MockClient{MockGet: test.NewMockGetFn(errors.New("boom"))},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockClaim{MockPortableClassReferencer: MockPortableClassReferencer{Ref: &corev1.LocalObjectReference{}}},
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: false,
		},
		"IncorrectNonPortableClassKind": {
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
					switch o := o.(type) {
					case *MockPortableClass:
						pc := &MockPortableClass{}
						pc.SetNonPortableClassReference(&corev1.ObjectReference{})
						*o = *pc
						return nil
					default:
						return errUnexpected
					}
				}),
			},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockClaim{MockPortableClassReferencer: MockPortableClassReferencer{Ref: &corev1.LocalObjectReference{}}},
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: false,
		},
		"CorrectNonPortableClassKind": {
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
					switch o := o.(type) {
					case *MockPortableClass:
						pc := &MockPortableClass{}
						version, kind := MockGVK(&MockNonPortableClass{}).ToAPIVersionAndKind()
						pc.SetNonPortableClassReference(&corev1.ObjectReference{
							Kind:       kind,
							APIVersion: version,
						})
						*o = *pc
						return nil
					default:
						return errUnexpected
					}
				}),
			},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockClaim{MockPortableClassReferencer: MockPortableClassReferencer{Ref: &corev1.LocalObjectReference{}}},
			kind: ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := HasIndirectClassReferenceKind(tc.c, tc.s, tc.kind)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HasIndirectClassReferenceKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNoPortableClassReference(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAClassReferencer": {
			want: false,
		},
		"NoClassReference": {
			obj:  &MockClaim{},
			want: true,
		},
		"HasClassReference": {
			obj:  &MockClaim{MockPortableClassReferencer: MockPortableClassReferencer{Ref: &corev1.LocalObjectReference{}}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NoPortableClassReference()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NoClassReference(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNoMangedResourceReference(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAManagedResourceReferencer": {
			want: false,
		},
		"NoManagedResourceReference": {
			obj:  &MockClaim{},
			want: true,
		},
		"HasClassReference": {
			obj:  &MockClaim{MockManagedResourceReferencer: MockManagedResourceReferencer{Ref: &corev1.ObjectReference{}}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NoManagedResourceReference()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NoManagedResourecReference(...): -want, +got:\n%s", diff)
			}
		})
	}
}
