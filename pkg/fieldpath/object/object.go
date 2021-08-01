/*
Copyright 2021 The Crossplane Authors.

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

package object

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// ToPaved tries to convert a runtime.Object into a *Paved via
// runtime.DefaultUnstructuredConverter if needed. Returns the paved if
// the conversion is successful along with whether the
// runtime.DefaultUnstructuredConverter has been employed during the
// conversion.
func ToPaved(o runtime.Object) (*fieldpath.Paved, bool, error) {
	if u, ok := o.(interface{ UnstructuredContent() map[string]interface{} }); ok {
		return fieldpath.Pave(u.UnstructuredContent()), false, nil
	}

	oMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, false, err
	}
	return fieldpath.Pave(oMap), true, nil
}

// PatchFieldValueToObject applies the value to the "to" object at the given
// path with the given merge options, returning any errors as they occur.
// If no merge options is supplied, then destination field is replaced
// with the given value.
func PatchFieldValueToObject(fieldPath string, value interface{}, to runtime.Object, mo *v1.MergeOptions) error {
	paved, copied, err := ToPaved(to)
	if err != nil {
		return err
	}

	if err := paved.MergeValue(fieldPath, value, mo); err != nil {
		return err
	}

	if copied {
		return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
	}
	return nil
}

// MergePath merges the value at the given field path of the src object into
// the dst object.
func MergePath(path string, dst, src runtime.Object, mergeOptions *v1.MergeOptions) error {
	srcPaved, _, err := ToPaved(src)
	if err != nil {
		return err
	}

	val, err := srcPaved.GetValue(path)
	// if src has no value at the specified path, then nothing to merge
	if fieldpath.IsNotFound(err) || val == nil {
		return nil
	}
	if err != nil {
		return err
	}

	return PatchFieldValueToObject(path, val, dst, mergeOptions)
}

// MergeReplace merges the value at path from dst into
// a copy of src and then replaces the value at path of
// dst with the merged value. src object is not modified.
func MergeReplace(path string, src, dst runtime.Object, mo *v1.MergeOptions) error {
	copySrc := src.DeepCopyObject()
	if err := MergePath(path, copySrc, dst, mo); err != nil {
		return err
	}
	// replace desired object's value at fieldPath with
	// the computed (merged) current value at the same path
	return MergePath(path, dst, copySrc, nil)
}
