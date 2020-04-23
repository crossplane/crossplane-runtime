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

package unstructured

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestObjectKinds(t *testing.T) {
	want := schema.GroupVersionKind{
		Group:   "g",
		Version: "v",
		Kind:    "k",
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(want)

	ot := ObjectTyper{}
	got, _, _ := ot.ObjectKinds(u)

	if diff := cmp.Diff([]schema.GroupVersionKind{want}, got); diff != "" {
		t.Errorf("ot.ObjectKinds(...): -want, +got\n%s", diff)
	}
}
