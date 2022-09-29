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
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

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

func GeneratePatchesFromRaw(raw []byte) (FieldPatches, error) {
	u := &unstructured.Unstructured{Object: make(map[string]any)}
	if err := json.Unmarshal(raw, u); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal raw data")
	}
	return GeneratePatches(u.Object, "")
}

type FieldPatch struct {
	FieldPath string
	Value     any
}

func GeneratePatches(o map[string]any, prefix string) (FieldPatches, error) {
	var result []FieldPatch
	for key, val := range o {
		switch v := val.(type) {
		case string, []string, int64, []int64, float64, []float64, bool, []bool:
			result = append(result, FieldPatch{
				FieldPath: strings.TrimPrefix(prefix+"."+key, "."),
				Value:     v,
			})
		case map[string]any:
			path := strings.TrimPrefix(prefix+"."+key, ".")
			ps, err := GeneratePatches(v, path)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot generate patches for %s", path)
			}
			result = append(result, ps...)
		case []map[string]any:
			for i, m := range v {
				path := strings.TrimPrefix(fmt.Sprintf("%s[%d]", prefix, i), ".")
				ps, err := GeneratePatches(m, path)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot generate patches for %s", path)
				}
				result = append(result, ps...)
			}
		default:
			return nil, errors.Errorf("unsupported type %s at path %s", reflect.TypeOf(v), strings.TrimPrefix(prefix+"."+key, "."))
		}
	}
	return result, nil
}
