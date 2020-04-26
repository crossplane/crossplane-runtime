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

package requirement

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

func TestWithGroupVersionKind(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "g",
		Version: "v1",
		Kind:    "k",
	}
	cases := map[string]struct {
		gvk  schema.GroupVersionKind
		want *Unstructured
	}{
		"New": {
			gvk: gvk,
			want: &Unstructured{Unstructured: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "g/v1",
					"kind":       "k",
				},
			},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := New(WithGroupVersionKind(tc.gvk))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("New(WithGroupVersionKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditions(t *testing.T) {
	cases := map[string]struct {
		reason string
		u      *Unstructured
		set    []v1alpha1.Condition
		get    v1alpha1.ConditionType
		want   v1alpha1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to set a condition of an empty Unstructured.",
			u:      New(),
			set:    []v1alpha1.Condition{v1alpha1.Available(), v1alpha1.ReconcileSuccess()},
			get:    v1alpha1.TypeReady,
			want:   v1alpha1.Available(),
		},
		"ExistingCondition": {
			reason: "It should be possible to overwrite a condition that is already set.",
			u:      New(WithConditions(v1alpha1.Creating())),
			set:    []v1alpha1.Condition{v1alpha1.Available()},
			get:    v1alpha1.TypeReady,
			want:   v1alpha1.Available(),
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Unstructured{unstructured.Unstructured{Object: map[string]interface{}{
				"status": "wat",
			}}},
			set:  []v1alpha1.Condition{v1alpha1.Available()},
			get:  v1alpha1.TypeReady,
			want: v1alpha1.Condition{},
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &Unstructured{unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": "wat",
				},
			}}},
			set:  []v1alpha1.Condition{v1alpha1.Available()},
			get:  v1alpha1.TypeReady,
			want: v1alpha1.Available(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConditions(tc.set...)
			got := tc.u.GetCondition(tc.get)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nu.GetCondition(%s): -want, +got:\n%s", tc.reason, tc.get, diff)
			}
		})
	}
}

func TestCompositionSelector(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}}
	cases := map[string]struct {
		u    *Unstructured
		set  *metav1.LabelSelector
		want *metav1.LabelSelector
	}{
		"NewSel": {
			u:    New(),
			set:  sel,
			want: sel,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionSelector(tc.set)
			got := tc.u.GetCompositionSelector()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionSelector(): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestCompositionReference(t *testing.T) {
	ref := &corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *corev1.ObjectReference
		want *corev1.ObjectReference
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionReference(tc.set)
			got := tc.u.GetCompositionReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestResourceReference(t *testing.T) {
	ref := &corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *corev1.ObjectReference
		want *corev1.ObjectReference
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetResourceReference(tc.set)
			got := tc.u.GetResourceReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetResourceReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWriteConnectionSecretToReference(t *testing.T) {
	ref := &v1alpha1.LocalSecretReference{Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *v1alpha1.LocalSecretReference
		want *v1alpha1.LocalSecretReference
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetWriteConnectionSecretToReference(tc.set)
			got := tc.u.GetWriteConnectionSecretToReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetWriteConnectionSecretToReference(): -want, +got:\n%s", diff)
			}
		})
	}
}
