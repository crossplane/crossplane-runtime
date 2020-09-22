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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
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

func TestAllOf(t *testing.T) {
	cases := map[string]struct {
		fns  []PredicateFn
		obj  runtime.Object
		want bool
	}{
		"AllPredicatesPass": {
			fns: []PredicateFn{
				func(obj runtime.Object) bool { return true },
				func(obj runtime.Object) bool { return true },
			},
			want: true,
		},
		"NoPredicatesPass": {
			fns: []PredicateFn{
				func(obj runtime.Object) bool { return false },
				func(obj runtime.Object) bool { return false },
			},
			want: false,
		},
		"SomePredicatesPass": {
			fns: []PredicateFn{
				func(obj runtime.Object) bool { return false },
				func(obj runtime.Object) bool { return true },
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := AllOf(tc.fns...)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("AllOf(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsManagedKind(t *testing.T) {
	cases := map[string]struct {
		kind ManagedKind
		ot   runtime.ObjectTyper
		obj  runtime.Object
		want bool
	}{
		"IsKind": {
			kind: ManagedKind(fake.GVK(&fake.Managed{})),
			ot:   MockTyper{GVKs: []schema.GroupVersionKind{fake.GVK(&fake.Managed{})}},
			want: true,
		},
		"IsNotKind": {
			kind: ManagedKind(fake.GVK(&fake.Managed{})),
			ot:   MockTyper{GVKs: []schema.GroupVersionKind{fake.GVK(&fake.Object{})}},
			want: false,
		},
		"ErrorDeterminingKind": {
			kind: ManagedKind(fake.GVK(&fake.Managed{})),
			ot:   MockTyper{Error: errors.New("boom")},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsManagedKind(tc.kind, tc.ot)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsManagedKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsControlledByKind(t *testing.T) {
	controller := true

	cases := map[string]struct {
		kind schema.GroupVersionKind
		obj  runtime.Object
		want bool
	}{
		"NoObjectMeta": {
			want: false,
		},
		"NoControllerRef": {
			obj:  &corev1.Secret{},
			want: false,
		},
		"WrongAPIVersion": {
			kind: fake.GVK(&fake.Managed{}),
			obj: &corev1.Secret{ObjectMeta: v1.ObjectMeta{OwnerReferences: []v1.OwnerReference{
				{
					Kind:       fake.GVK(&fake.Managed{}).Kind,
					Controller: &controller,
				},
			}}},
			want: false,
		},
		"WrongKind": {
			kind: fake.GVK(&fake.Managed{}),
			obj: &corev1.Secret{ObjectMeta: v1.ObjectMeta{OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
					Controller: &controller,
				},
			}}},
			want: false,
		},
		"IsControlledByKind": {
			kind: fake.GVK(&fake.Managed{}),
			obj: &corev1.Secret{ObjectMeta: v1.ObjectMeta{OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
					Kind:       fake.GVK(&fake.Managed{}).Kind,
					Controller: &controller,
				},
			}}},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsControlledByKind(tc.kind)(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsControlledByKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsPropagator(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAnAnnotator": {
			want: false,
		},
		"NotAPropagator": {
			obj: &corev1.Secret{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				"some.annotation": "someValue",
			}}},
			want: false,
		},
		"IsPropagator": {
			obj: func() runtime.Object {
				o := &fake.Object{}
				o.SetNamespace("somenamespace")
				o.SetName("somename")
				mg := &fake.Managed{}
				meta.AllowPropagation(mg, o)
				return mg
			}(),
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsPropagator()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsPropagator(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsPropagated(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotAnAnnotator": {
			want: false,
		},
		"NotPropagated": {
			obj: &corev1.Secret{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				"some.annotation": "someValue",
			}}},
			want: false,
		},
		"IsPropagated": {
			obj: func() runtime.Object {
				o := &fake.Object{}
				mg := &fake.Managed{}
				mg.SetNamespace("somenamespace")
				mg.SetName("somename")
				meta.AllowPropagation(mg, o)
				return o
			}(),
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsPropagated()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("IsPropagated(...): -want, +got:\n%s", diff)
			}
		})
	}
}
