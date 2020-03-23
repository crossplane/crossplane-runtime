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

package v1alpha1

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

const (
	errMathNoMultiplier   = "no input is given"
	errMathInputNonNumber = "input is required to be a number for math transformer"
)

var (
	errTransformAtIndex  = func(i int) string { return fmt.Sprintf("transform at index %d returned error", i) }
	errTypeNotSupported  = func(s string) string { return fmt.Sprintf("transform type %s is not supported", s) }
	errConfigMissing     = func(s string) string { return fmt.Sprintf("given type %s requires configuration", s) }
	errTransformWithType = func(s string) string { return fmt.Sprintf("%s transform could not resolve", s) }

	errMapNotFound         = func(s string) string { return fmt.Sprintf("given value %s is not found in map", s) }
	errMapTypeNotSupported = func(s string) string { return fmt.Sprintf("type %s is not supported for map transform", s) }
)

// CompositionPatch is used to patch the field on the base resource at ToFieldPath
// after piping the value that is at FromFieldPath of the target resource through
// transformers.
type CompositionPatch struct {
	FromFieldPath string      `json:"fromFieldPath"`
	ToFieldPath   string      `json:"toFieldPath,omitempty"`
	Transforms    []Transform `json:"transforms,omitempty"`
}

func (c *CompositionPatch) Patch(base, target *fieldpath.Paved) error {
	in, err := base.GetValue(c.FromFieldPath)
	if err != nil {
		return err
	}
	out := in
	for i, f := range c.Transforms {
		out, err = f.Transform(out)
		if err != nil {
			return errors.Wrap(err, errTransformAtIndex(i))
		}
	}
	return target.SetValue(c.ToFieldPath, out)
}

// TODO(muvaf): Reconsider the usefulness of Transformer interface. Nothing
// implements it outside the package and its use helps only in Transform function
// since actual Transformers are strong-typed with jsontags anyway.

// Transformers resolve arbitrary input type to arbitrary output type. The
// reasoning for this loose typing is that a Transformer may have an input of type
// A but output of type B; given that there will be many pairs like this, it makes
// more sense to enforce types at the lowest level of the chain which is content
// of actual Resolve functions of individual transformers.
type Transformer interface {
	Resolve(input interface{}) (interface{}, error)
}

type Transform struct {
	Type string         `json:"type"`
	Math *MathTransform `json:"math,omitempty"`
	Map  *MapTransform  `json:"map,omitempty"`
}

func (t *Transform) Transform(input interface{}) (interface{}, error) {
	var transformer Transformer
	switch t.Type {
	case "math":
		transformer = t.Math
	case "map":
		transformer = t.Map
	default:
		return 0, errors.New(errTypeNotSupported(t.Type))
	}
	if transformer == nil {
		return nil, errors.New(errConfigMissing(t.Type))
	}
	out, err := transformer.Resolve(input)
	return out, errors.Wrap(err, errTransformWithType(t.Type))
}

type MathTransform struct {
	Multiply *int64 `json:"multiply,omitempty"`
}

func (m *MathTransform) Resolve(input interface{}) (interface{}, error) {
	if m.Multiply == nil {
		return nil, errors.New(errMathNoMultiplier)
	}
	switch i := input.(type) {
	case int64:
		return *m.Multiply * i, nil
	case int:
		return *m.Multiply * int64(i), nil
	default:
		return nil, errors.New(errMathInputNonNumber)
	}
}

type MapTransform struct {
	Pairs map[string]string `json:",inline"`
}

func (m *MapTransform) Resolve(input interface{}) (interface{}, error) {
	switch i := input.(type) {
	case string:
		val, ok := m.Pairs[i]
		if !ok {
			return nil, errors.New(errMapNotFound(i))
		}
		return val, nil
	default:
		return nil, errors.New(errMapTypeNotSupported(reflect.TypeOf(input).String()))
	}
}