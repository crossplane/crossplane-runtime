/*
Copyright 2022 The Crossplane Authors.

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
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
)

// PatchFields is an ApplyOption that can be used to generate a patch set from
// desired object and apply every statement one by one to the current object.
// It treats array as maps whose key is their index.
func PatchFields(_ context.Context, c, d runtime.Object) error {
	du, ok := d.(*composed.Unstructured)
	if !ok {
		return errors.New("current object is not of type unstructured")
	}
	fps, err := GeneratePatches(du.Object, "")
	if err != nil {
		return errors.Wrap(err, "cannot generate patches from desired object")
	}

	// We'd like to override all fields in the current object with what we want
	// them to be before updating.
	cu, ok := c.(*unstructured.Unstructured)
	if !ok {
		return errors.New("current object is not of type Unstructured")
	}
	cu.DeepCopyInto(du.GetUnstructured())
	return errors.Wrap(fps.Apply(du.GetUnstructured()), "cannot apply field patches")
}

type FieldPatches []FieldPatch

func (fps FieldPatches) Apply(o *unstructured.Unstructured) error {
	obj := fieldpath.Pave(o.Object)
	for _, p := range fps {
		if err := obj.SetValue(p.FieldPath, p.Value); err != nil {
			return errors.Wrapf(err, "cannot set value %s to field path %s", p.Value, p.FieldPath)
		}
	}
	return nil
}

type FieldPatch struct {
	FieldPath string
	Value     any
}

func GeneratePatches(o any, fieldPath string) (FieldPatches, error) {
	var result []FieldPatch
	switch v := o.(type) {
	case string, []string, int64, []int64, float64, []float64, bool, []bool:
		result = append(result, FieldPatch{
			FieldPath: fieldPath,
			Value:     v,
		})
	case map[string]any:
		for key, val := range v {
			path := join(fieldPath, key)
			ps, err := GeneratePatches(val, path)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot generate patches for %s", path)
			}
			result = append(result, ps...)
		}
	case []map[string]any:
		for i, m := range v {
			path := joinElem(fieldPath, i)
			ps, err := GeneratePatches(m, path)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot generate patches for %s", path)
			}
			result = append(result, ps...)
		}
	case []any:
		for i, el := range v {
			path := joinElem(fieldPath, i)
			ps, err := GeneratePatches(el, path)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot generate patches for %s", path)
			}
			result = append(result, ps...)
		}
	default:
		return nil, errors.Errorf("unsupported type %s at path %s", reflect.TypeOf(v), fieldPath)
	}
	return result, nil
}

func join(prefix, key string) string {
	if key == "" {
		return prefix
	}
	if prefix == "" {
		return key
	}
	if strings.Contains(key, ".") {
		return fmt.Sprintf("%s[%s]", prefix, key)
	}
	return fmt.Sprintf("%s.%s", prefix, key)
}

func joinElem(prefix string, i int) string {
	return fmt.Sprintf("%s[%d]", strings.Trim(prefix, "."), i)
}
