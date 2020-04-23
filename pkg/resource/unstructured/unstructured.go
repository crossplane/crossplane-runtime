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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// An ObjectTyper returns the GroupVersionKind that a runtime.Object considers
// itself to have.
type ObjectTyper struct{}

// ObjectKinds returns one GroupVersionKind of the supplied object. It always
// return false and nil.
func (t ObjectTyper) ObjectKinds(o runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	return []schema.GroupVersionKind{o.GetObjectKind().GroupVersionKind()}, false, nil

}

// Recognizes always returns true.
func (t ObjectTyper) Recognizes(gvk schema.GroupVersionKind) bool { return true }
