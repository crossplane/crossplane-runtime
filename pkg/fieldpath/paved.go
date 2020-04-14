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

package fieldpath

import (
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A Paved JSON object supports getting and setting values by their field path.
type Paved struct {
	object map[string]interface{}
}

// Pave a JSON object, making it possible to get and set values by field path.
func Pave(object map[string]interface{}) *Paved {
	return &Paved{object: object}
}

// PaveObject paves a runtime.Object, making it possible to get and set values
// by field path.
func PaveObject(o runtime.Object) (*Paved, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	return Pave(u), errors.Wrap(err, "")
}

// MarshalJSON to the underlying object.
func (p Paved) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.object)
}

// UnmarshalJSON from the underlying object.
func (p *Paved) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &p.object)
}

// UnstructuredContent returns the JSON serialisable content of this Paved.
func (p *Paved) UnstructuredContent() map[string]interface{} {
	if p.object == nil {
		return make(map[string]interface{})
	}
	return p.object
}

// SetUnstructuredContent sets the JSON serialisable content of this Paved.
func (p *Paved) SetUnstructuredContent(content map[string]interface{}) {
	p.object = content
}

func (p *Paved) getValue(s Segments) (interface{}, error) {
	var it interface{} = p.object
	for i, current := range s {
		final := i == len(s)-1
		switch current.Type {
		case SegmentIndex:
			array, ok := it.([]interface{})
			if !ok {
				return nil, errors.Errorf("%s: not an array", s[:i])
			}
			if int(current.Index) >= len(array) {
				return nil, errors.Errorf("%s: no such element", s[:i+1])
			}
			if final {
				return array[current.Index], nil
			}
			it = array[current.Index]

		case SegmentField:
			object, ok := it.(map[string]interface{})
			if !ok {
				return nil, errors.Errorf("%s: not an object", s[:i])
			}
			v, ok := object[current.Field]
			if !ok {
				return nil, errors.Errorf("%s: no such field", s[:i+1])
			}
			if final {
				return v, nil
			}
			it = object[current.Field]
		}
	}

	// This should be unreachable.
	return nil, nil
}

// GetValue of the supplied field path.
func (p *Paved) GetValue(path string) (interface{}, error) {
	segments, err := Parse(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse path %q", path)
	}

	return p.getValue(segments)
}

// GetString value of the supplied field path.
func (p *Paved) GetString(path string) (string, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return "", err
	}

	s, ok := v.(string)
	if !ok {
		return "", errors.Errorf("%s: not a string", path)
	}
	return s, nil
}

// GetStringArray value of the supplied field path.
func (p *Paved) GetStringArray(path string) ([]string, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return nil, err
	}

	a, ok := v.([]interface{})
	if !ok {
		return nil, errors.Errorf("%s: not an array", path)
	}

	sa := make([]string, len(a))
	for i := range a {
		s, ok := a[i].(string)
		if !ok {
			return nil, errors.Errorf("%s: not an array of strings", path)
		}
		sa[i] = s
	}

	return sa, nil
}

// GetStringObject value of the supplied field path.
func (p *Paved) GetStringObject(path string) (map[string]string, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return nil, err
	}

	o, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("%s: not an object", path)
	}

	so := make(map[string]string)
	for k, in := range o {
		s, ok := in.(string)
		if !ok {
			return nil, errors.Errorf("%s: not an object with string field values", path)
		}
		so[k] = s

	}

	return so, nil
}

// GetBool value of the supplied field path.
func (p *Paved) GetBool(path string) (bool, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return false, err
	}

	b, ok := v.(bool)
	if !ok {
		return false, errors.Errorf("%s: not a bool", path)
	}
	return b, nil
}

// GetNumber value of the supplied field path.
func (p *Paved) GetNumber(path string) (float64, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return 0, err
	}

	f, ok := v.(float64)
	if !ok {
		return 0, errors.Errorf("%s: not a (float64) number", path)
	}
	return f, nil
}

func (p *Paved) setValue(s Segments, value interface{}) error {
	// We expect p.object to look like JSON data that was unmarshalled into an
	// interface{} per https://golang.org/pkg/encoding/json/#Unmarshal. We
	// marshal our value to JSON and unmarshal it into an interface{} to ensure
	// it meets these criteria before setting it within p.object.
	var v interface{}
	j, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "cannot marshal value to JSON")
	}
	if err := json.Unmarshal(j, &v); err != nil {
		return errors.Wrap(err, "cannot unmarshal value from JSON")
	}

	var in interface{} = p.object
	for i, current := range s {
		final := i == len(s)-1

		switch current.Type {
		case SegmentIndex:
			array, ok := in.([]interface{})
			if !ok {
				return errors.Errorf("%s is not an array", s[:i])
			}

			if final {
				array[current.Index] = v
				return nil
			}

			prepareElement(array, current, s[i+1])
			in = array[current.Index]

		case SegmentField:
			object, ok := in.(map[string]interface{})
			if !ok {
				return errors.Errorf("%s is not an object", s[:i])
			}

			if final {
				object[current.Field] = v
				return nil
			}

			prepareField(object, current, s[i+1])
			in = object[current.Field]
		}
	}

	return nil
}

func prepareElement(array []interface{}, current, next Segment) {
	// If this segment is not the final one and doesn't exist we need to
	// create it for our next segment.
	if array[current.Index] == nil {
		switch next.Type {
		case SegmentIndex:
			array[current.Index] = make([]interface{}, next.Index+1)
		case SegmentField:
			array[current.Index] = make(map[string]interface{})
		}
		return
	}

	// If our next segment indexes an array that exists in our current segment's
	// element we must ensure the array is long enough to set the next segment.
	if next.Type != SegmentIndex {
		return
	}

	na, ok := array[current.Index].([]interface{})
	if !ok {
		return
	}

	if int(next.Index) < len(na) {
		return
	}

	array[current.Index] = append(na, make([]interface{}, int(next.Index)-len(na)+1)...)
}

func prepareField(object map[string]interface{}, current, next Segment) {
	// If this segment is not the final one and doesn't exist we need to
	// create it for our next segment.
	if _, ok := object[current.Field]; !ok {
		switch next.Type {
		case SegmentIndex:
			object[current.Field] = make([]interface{}, next.Index+1)
		case SegmentField:
			object[current.Field] = make(map[string]interface{})
		}
		return
	}

	// If our next segment indexes an array that exists in our current segment's
	// field we must ensure the array is long enough to set the next segment.
	if next.Type != SegmentIndex {
		return
	}

	na, ok := object[current.Field].([]interface{})
	if !ok {
		return
	}

	if int(next.Index) < len(na) {
		return
	}

	object[current.Field] = append(na, make([]interface{}, int(next.Index)-len(na)+1)...)
}

// SetValue at the supplied field path.
func (p *Paved) SetValue(path string, value interface{}) error {
	segments, err := Parse(path)
	if err != nil {
		return errors.Wrapf(err, "cannot parse path %q", path)
	}
	return p.setValue(segments, value)
}

// SetString value at the supplied field path.
func (p *Paved) SetString(path, value string) error {
	return p.SetValue(path, value)
}

// SetBool value at the supplied field path.
func (p *Paved) SetBool(path string, value bool) error {
	return p.SetValue(path, value)
}

// SetNumber value at the supplied field path.
func (p *Paved) SetNumber(path string, value float64) error {
	return p.SetValue(path, value)
}

// GetCondition of this Paved resource.
func (p *Paved) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	cs := &v1alpha1.ConditionedStatus{}

	v, err := p.GetValue("status")
	if err != nil {
		return cs.GetCondition(ct)
	}

	j, err := json.Marshal(v)
	if err != nil {
		return cs.GetCondition(ct)
	}

	// We want to call cs.GetCondition regardless of whether this unmarshal
	// succeeds; calling GetCondition on an empty *ConditionedStatus returns a
	// sane zero value.
	_ = json.Unmarshal(j, cs)
	return cs.GetCondition(ct)
}

// SetConditions of this Paved resource.
func (p *Paved) SetConditions(c ...v1alpha1.Condition) {
	cs := &v1alpha1.ConditionedStatus{Conditions: c}

	if _, err := p.GetValue("status"); err != nil {
		cs.SetConditions(c...)
		_ = p.SetValue("status", cs)
		return
	}

	v, err := p.GetValue("status.conditions")
	if err != nil {
		cs.SetConditions(c...)
		_ = p.SetValue("status.conditions", cs.Conditions)
		return
	}

	j, err := json.Marshal(v)
	if err != nil {
		return
	}

	if err := json.Unmarshal(j, &cs.Conditions); err != nil {
		// If status.conditions is currently something other than an array of
		// Condition we fail silently without replacing it.
		return
	}

	cs.SetConditions(c...)

	// This method silently swallows any error encountered while setting a
	// condition in the unstructured object in order to satisfy the
	// resource.Conditioned interface.
	_ = p.SetValue("status.conditions", cs.Conditions)
}
