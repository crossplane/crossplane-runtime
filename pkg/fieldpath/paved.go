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
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// DefaultMaxFieldPathIndex is the max allowed index in a field path.
const DefaultMaxFieldPathIndex = 1024

type errNotFound struct {
	error
}

func (e errNotFound) IsNotFound() bool {
	return true
}

// IsNotFound returns true if the supplied error indicates a field path was not
// found, for example because a field did not exist within an object or an
// index was out of bounds in an array.
func IsNotFound(err error) bool {
	cause := errors.Cause(err)
	_, ok := cause.(interface {
		IsNotFound() bool
	})
	return ok
}

// A Paved JSON object supports getting and setting values by their field path.
type Paved struct {
	object            map[string]interface{}
	maxFieldPathIndex uint
}

// PavedOption can be used to configure a Paved behavior.
type PavedOption func(paved *Paved)

// PaveObject paves a runtime.Object, making it possible to get and set values
// by field path. o must be a non-nil pointer to an object.
func PaveObject(o runtime.Object, opts ...PavedOption) (*Paved, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	return Pave(u, opts...), errors.Wrap(err, "cannot convert object to unstructured data")
}

// Pave a JSON object, making it possible to get and set values by field path.
func Pave(object map[string]interface{}, opts ...PavedOption) *Paved {
	p := &Paved{object: object, maxFieldPathIndex: DefaultMaxFieldPathIndex}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// WithMaxFieldPathIndex returns a PavedOption that sets the max allowed index for field paths, 0 means no limit.
func WithMaxFieldPathIndex(max uint) PavedOption {
	return func(paved *Paved) {
		paved.maxFieldPathIndex = max
	}
}

func (p *Paved) maxFieldPathIndexEnabled() bool {
	return p.maxFieldPathIndex > 0
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
	return getValueFromInterface(p.object, s)
}

func getValueFromInterface(it interface{}, s Segments) (interface{}, error) {
	for i, current := range s {
		final := i == len(s)-1
		switch current.Type {
		case SegmentIndex:
			array, ok := it.([]interface{})
			if !ok {
				return nil, errors.Errorf("%s: not an array", s[:i])
			}
			if int(current.Index) >= len(array) {
				return nil, errNotFound{errors.Errorf("%s: no such element", s[:i+1])}
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
				return nil, errNotFound{errors.Errorf("%s: no such field", s[:i+1])}
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

// ExpandWildcards expands wildcards for a given field path. It returns an
// array of field paths with expanded values. Please note that expanded paths
// depend on the input data which is paved.object.
//
// Example:
//
// For a Paved object with the following data: []byte(`{"spec":{"containers":[{"name":"cool", "image": "latest", "args": ["start", "now", "debug"]}]}}`),
// ExpandWildcards("spec.containers[*].args[*]") returns:
// []string{"spec.containers[0].args[0]", "spec.containers[0].args[1]", "spec.containers[0].args[2]"},
func (p *Paved) ExpandWildcards(path string) ([]string, error) {
	segments, err := Parse(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse path %q", path)
	}
	segmentsArray, err := expandWildcards(p.object, segments)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot expand wildcards for segments: %q", segments)
	}
	paths := make([]string, len(segmentsArray))
	for i, s := range segmentsArray {
		paths[i] = s.String()
	}
	return paths, nil
}

// Note(turkenh): Explanation for nolint:gocyclo
// Even complexity turns out to be high, it is mostly because we have duplicate
// logic for arrays and maps and a couple of error handling.
func expandWildcards(data interface{}, segments Segments) ([]Segments, error) { //nolint:gocyclo
	var res []Segments
	it := data
	for i, current := range segments {
		// wildcards are regular fields with "*" as string
		if current.Type == SegmentField && current.Field == wildcard {
			switch mapOrArray := it.(type) {
			case []interface{}:
				for ix := range mapOrArray {
					expanded := make(Segments, len(segments))
					copy(expanded, segments)
					expanded = append(append(expanded[:i], FieldOrIndex(strconv.Itoa(ix))), expanded[i+1:]...)
					r, err := expandWildcards(data, expanded)
					if err != nil {
						return nil, errors.Wrapf(err, "%q: cannot expand wildcards", expanded)
					}
					res = append(res, r...)
				}
			case map[string]interface{}:
				for k := range mapOrArray {
					expanded := make(Segments, len(segments))
					copy(expanded, segments)
					expanded = append(append(expanded[:i], Field(k)), expanded[i+1:]...)
					r, err := expandWildcards(data, expanded)
					if err != nil {
						return nil, errors.Wrapf(err, "%q: cannot expand wildcards", expanded)
					}
					res = append(res, r...)
				}
			default:
				return nil, errors.Errorf("%q: unexpected wildcard usage", segments[:i])
			}
			return res, nil
		}
		var err error
		it, err = getValueFromInterface(data, segments[:i+1])
		if IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	return append(res, segments), nil
}

// GetValue of the supplied field path.
func (p *Paved) GetValue(path string) (interface{}, error) {
	segments, err := Parse(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse path %q", path)
	}

	return p.getValue(segments)
}

// GetValueInto the supplied type.
func (p *Paved) GetValueInto(path string, out interface{}) error {
	val, err := p.GetValue(path)
	if err != nil {
		return err
	}
	js, err := json.Marshal(val)
	if err != nil {
		return errors.Wrap(err, "cannot marshal value to JSON")
	}
	return errors.Wrap(json.Unmarshal(js, out), "cannot unmarshal value from JSON")
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

// NOTE(muvaf): If there is no CRD, unstructured.Unstructured reads numbers as
// float64. However, in practice, use of float64 is discouraged and when you fetch
// an instance of a CRD whose number fields are int64, you'll get int64. So,
// it's not really possible to test this without an api-server but that's the
// actual behavior.

// GetNumber value of the supplied field path.
// Deprecated: Use of float64 is discouraged. Please use GetInteger.
// See https://github.com/kubernetes/community/blob/c9ae475/contributors/devel/sig-architecture/api-conventions.md#primitive-types
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

// GetInteger value of the supplied field path.
func (p *Paved) GetInteger(path string) (int64, error) {
	v, err := p.GetValue(path)
	if err != nil {
		return 0, err
	}

	f, ok := v.(int64)
	if !ok {
		return 0, errors.Errorf("%s: not a (int64) number", path)
	}
	return f, nil
}

func (p *Paved) setValue(s Segments, value interface{}) error {
	// We expect p.object to look like JSON data that was unmarshalled into an
	// interface{} per https://golang.org/pkg/encoding/json/#Unmarshal. We
	// marshal our value to JSON and unmarshal it into an interface{} to ensure
	// it meets these criteria before setting it within p.object.
	v, err := toValidJSON(value)
	if err != nil {
		return err
	}

	if err := p.validateSegments(s); err != nil {
		return err
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

func toValidJSON(value interface{}) (interface{}, error) {
	var v interface{}
	j, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal value to JSON")
	}
	if err := json.Unmarshal(j, &v); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal value from JSON")
	}
	return v, nil
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

func (p *Paved) validateSegments(s Segments) error {
	if !p.maxFieldPathIndexEnabled() {
		return nil
	}
	for _, segment := range s {
		if segment.Type == SegmentIndex && segment.Index > p.maxFieldPathIndex {
			return errors.Errorf("index %v is greater than max allowed index %d", segment.Index, p.maxFieldPathIndex)
		}
	}
	return nil
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
