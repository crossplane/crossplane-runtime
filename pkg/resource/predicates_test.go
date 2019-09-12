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

type mockObject struct{ runtime.Object }

type mockPortableClassReferencer struct {
	runtime.Object
	ref *corev1.LocalObjectReference
}

func (r *mockPortableClassReferencer) GetPortableClassReference() *corev1.LocalObjectReference {
	return r.ref
}
func (r *mockPortableClassReferencer) SetPortableClassReference(_ *corev1.LocalObjectReference) {}

type mockManagedResourceReferencer struct {
	runtime.Object
	ref *corev1.ObjectReference
}

func (r *mockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference  { return r.ref }
func (r *mockManagedResourceReferencer) SetResourceReference(_ *corev1.ObjectReference) {}

func TestHasClassReferenceKinds(t *testing.T) {
	errBoom := errors.New("boom")
	errUnexpected := errors.New("unexpected object type")
	ck := ClassKinds{Portable: MockGVK(&MockPortableClass{}), NonPortable: MockGVK(&MockNonPortableClass{})}

	mockClaimWithRef := MockClaim{}
	mockClaimWithRef.SetPortableClassReference(&corev1.LocalObjectReference{Name: "cool-portable"})

	cases := map[string]struct {
		obj  runtime.Object
		c    client.Client
		s    runtime.ObjectCreater
		kind ClassKinds
		want bool
	}{
		"NotAClaim": {
			c:    &test.MockClient{},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &mockObject{},
			kind: ck,
			want: false,
		},
		"NoPortableClassReference": {
			c:    &test.MockClient{},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &MockClaim{},
			kind: ck,
			want: false,
		},
		"GetPortableClassError": {
			c:    &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			s:    MockSchemeWith(&MockClaim{}, &MockPortableClass{}, &MockNonPortableClass{}),
			obj:  &mockClaimWithRef,
			kind: ck,
			want: false,
		},
		"PortableClassHasReferenceIncorrectKind": {
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
			obj:  &mockClaimWithRef,
			kind: ck,
			want: false,
		},
		"HasCorrect": {
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
			obj:  &mockClaimWithRef,
			kind: ck,
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := HasClassReferenceKinds(tc.c, tc.s, tc.kind)
			got := fn(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HasClassReferenceKind(...): -want, +got:\n%s", diff)
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
			obj:  &mockObject{},
			want: false,
		},
		"NoClassReference": {
			obj:  &mockPortableClassReferencer{},
			want: true,
		},
		"HasClassReference": {
			obj:  &mockPortableClassReferencer{ref: &corev1.LocalObjectReference{Name: "cool"}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := NoPortableClassReference()
			got := fn(tc.obj)
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
		"NotAMangedResourceReferencer": {
			obj:  &mockObject{},
			want: false,
		},
		"NoManagedResourceReference": {
			obj:  &mockManagedResourceReferencer{},
			want: true,
		},
		"HasClassReference": {
			obj:  &mockManagedResourceReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := NoManagedResourceReference()
			got := fn(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NoManagedResourecReference(...): -want, +got:\n%s", diff)
			}
		})
	}
}
