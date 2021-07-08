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

package fieldpath

import (
	"reflect"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	errInvalidMerge = "failed to merge values"
)

// MergeOptions Specifies merge options on a field path
type MergeOptions struct { // TODO(aru): add more options that control merging behavior
	// Specifies that already existing values in a merged map should be preserved
	KeepMapValues bool `json:"keepMapValues,omitempty"`
	// Specifies that already existing elements in a merged slice should be preserved
	AppendSlice bool `json:"appendSlice,omitempty"`
}

// MergoConfiguration the default behavior is to replace maps and slices
func (mo *MergeOptions) MergoConfiguration() []func(*mergo.Config) {
	config := []func(*mergo.Config){mergo.WithOverride}
	if mo == nil {
		return config
	}

	if mo.KeepMapValues {
		config = config[:0]
	}
	if mo.AppendSlice {
		config = append(config, mergo.WithAppendSlice)
	}
	return config
}

// ToPaved tries to convert a runtime.Object into a *Paved via
// runtime.DefaultUnstructuredConverter if needed. Returns the paved if
// the conversion is successful along with whether the
// runtime.DefaultUnstructuredConverter has been employed during the
// conversion.
func ToPaved(o runtime.Object) (*Paved, bool, error) {
	if u, ok := o.(interface{ UnstructuredContent() map[string]interface{} }); ok {
		return Pave(u.UnstructuredContent()), false, nil
	}

	oMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, false, err
	}
	return Pave(oMap), true, nil
}

// PatchFieldValueToObject applies the value to the "to" object at the given
// path with the given merge options, returning any errors as they occur.
// If no merge options is supplied, then destination field is replaced
// with the given value.
func PatchFieldValueToObject(fieldPath string, value interface{}, to runtime.Object,
	mergeOptions *MergeOptions) error {
	paved, copied, err := ToPaved(to)
	if err != nil {
		return err
	}
	dst, err := paved.GetValue(fieldPath)
	if IsNotFound(err) || mergeOptions == nil {
		dst = nil
	} else if err != nil {
		return err
	}

	dst, err = merge(dst, value, mergeOptions)
	if err != nil {
		return err
	}

	if err := paved.SetValue(fieldPath, dst); err != nil {
		return err
	}

	if copied {
		return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.object, to)
	}
	return nil
}

// MergePath merges the value at the given field path of the src object into
// the dst object.
func MergePath(fieldPath string, dst, src runtime.Object, mergeOptions *MergeOptions) error {
	srcPaved, _, err := ToPaved(src)
	if err != nil {
		return err
	}

	val, err := srcPaved.GetValue(fieldPath)
	// if src has no value at the specified path, then nothing to merge
	if IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return PatchFieldValueToObject(fieldPath, val, dst, mergeOptions)
}

// merges the given src onto the given dst.
// If a nil merge options is supplied, the default behavior is MergeOptions'
// default behavior. If dst or src is nil, src is returned
// (i.e., dst replaced by src).
func merge(dst, src interface{}, mergeOptions *MergeOptions) (interface{}, error) {
	if dst == nil || src == nil {
		return src, nil // no merge, replace
	}

	m, ok := dst.(map[string]interface{})
	if reflect.TypeOf(src).Kind() != reflect.Map || !ok {
		return src, nil // not a map nor a struct, mergo cannot merge
	}

	// use merge semantics with the configured merge options to obtain the target dst value
	if err := mergo.Merge(&m, src, mergeOptions.MergoConfiguration()...); err != nil {
		return nil, errors.Wrap(err, errInvalidMerge)
	}
	return m, nil
}
