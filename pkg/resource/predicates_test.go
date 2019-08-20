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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockObject struct{ runtime.Object }

type mockClassReferencer struct {
	runtime.Object
	ref *corev1.ObjectReference
}

func (r *mockClassReferencer) GetClassReference() *corev1.ObjectReference  { return r.ref }
func (r *mockClassReferencer) SetClassReference(_ *corev1.ObjectReference) {}

type mockManagedResourceReferencer struct {
	runtime.Object
	ref *corev1.ObjectReference
}

func (r *mockManagedResourceReferencer) GetResourceReference() *corev1.ObjectReference  { return r.ref }
func (r *mockManagedResourceReferencer) SetResourceReference(_ *corev1.ObjectReference) {}

func TestHasClassReferenceKind(t *testing.T) {
	ck := ClassKind(MockGVK(&MockClass{}))

	cases := map[string]struct {
		obj  runtime.Object
		kind ClassKind
		want bool
	}{
		"NotAClassReferencer": {
			obj:  &mockObject{},
			kind: ck,
			want: false,
		},
		"NoClassReference": {
			obj:  &mockClassReferencer{},
			kind: ck,
			want: false,
		},
		"HasClassReferenceIncorrectKind": {
			obj:  &mockClassReferencer{ref: &corev1.ObjectReference{}},
			kind: ck,
			want: false,
		},
		"HasClassReferenceCorrectKind": {
			obj:  &mockClassReferencer{ref: &corev1.ObjectReference{Kind: ck.Kind, APIVersion: schema.GroupVersion{Group: ck.Group, Version: ck.Version}.String()}},
			kind: ck,
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := HasClassReferenceKind(tc.kind)
			got := fn(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HasClassReferenceKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNoClassReference(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAClassReferencer": {
			obj:  &mockObject{},
			want: false,
		},
		"NoClassReference": {
			obj:  &mockClassReferencer{},
			want: true,
		},
		"HasClassReference": {
			obj:  &mockClassReferencer{ref: &corev1.ObjectReference{Name: "cool"}},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := NoClassReference()
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
