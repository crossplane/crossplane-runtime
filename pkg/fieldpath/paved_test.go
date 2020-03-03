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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestGetValue(t *testing.T) {
	type want struct {
		value interface{}
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataName": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				value: "cool",
			},
		},
		"ContainerName": {
			reason: "It should be possible to get a field from an object array element",
			path:   "spec.containers[0].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				value: "cool",
			},
		},
		"NestedArray": {
			reason: "It should be possible to get a field from a nested array",
			path:   "items[0][1]",
			data:   []byte(`{"items":[["a", "b"]]}`),
			want: want{
				value: "b",
			},
		},
		"OwnerRefController": {
			reason: "Requesting a boolean field path should work.",
			path:   "metadata.ownerRefs[0].controller",
			data:   []byte(`{"metadata":{"ownerRefs":[{"controller": true}]}}`),
			want: want{
				value: true,
			},
		},
		"MetadataVersion": {
			reason: "Requesting a number field should work",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				value: float64(2),
			},
		},
		"MetadataNope": {
			reason: "Requesting a non-existent object field should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"nope":"cool"}}`),
			want: want{
				err: errors.New("metadata.name: no such field"),
			},
		},
		"InsufficientContainers": {
			reason: "Requesting a non-existent array element should fail",
			path:   "spec.containers[1].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: errors.New("spec.containers[1]: no such element"),
			},
		},
		"NotAnArray": {
			reason: "Indexing an object should fail",
			path:   "metadata[1]",
			data:   []byte(`{"metadata":{"nope":"cool"}}`),
			want: want{
				err: errors.New("metadata: not an array"),
			},
		},
		"NotAnObject": {
			reason: "Requesting a field in an array should fail",
			path:   "spec.containers[nope].name",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			want: want{
				err: errors.New("spec.containers: not an object"),
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetValue(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetValue(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetValue(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	type want struct {
		value string
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataName": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				value: "cool",
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAString": {
			reason: "Requesting an non-string field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not a string"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetString(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetString(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetString(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetStringArray(t *testing.T) {
	type want struct {
		value []string
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataLabels": {
			reason: "It should be possible to get a field from a nested object",
			path:   "spec.containers[0].command",
			data:   []byte(`{"spec": {"containers": [{"command": ["/bin/bash"]}]}}`),
			want: want{
				value: []string{"/bin/bash"},
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAnArray": {
			reason: "Requesting an non-object field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not an array"),
			},
		},
		"NotAStringArray": {
			reason: "Requesting an non-string-object field path should fail",
			path:   "metadata.versions",
			data:   []byte(`{"metadata":{"versions":[1,2]}}`),
			want: want{
				err: errors.New("metadata.versions: not an array of strings"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetStringArray(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetStringArray(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetStringArray(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetStringObject(t *testing.T) {
	type want struct {
		value map[string]string
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataLabels": {
			reason: "It should be possible to get a field from a nested object",
			path:   "metadata.labels",
			data:   []byte(`{"metadata":{"labels":{"cool":"true"}}}`),
			want: want{
				value: map[string]string{"cool": "true"},
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotAnObject": {
			reason: "Requesting an non-object field path should fail",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				err: errors.New("metadata.version: not an object"),
			},
		},
		"NotAStringObject": {
			reason: "Requesting an non-string-object field path should fail",
			path:   "metadata.versions",
			data:   []byte(`{"metadata":{"versions":{"a": 2}}}`),
			want: want{
				err: errors.New("metadata.versions: not an object with string field values"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetStringObject(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetStringObject(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetStringObject(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	type want struct {
		value bool
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"OwnerRefController": {
			reason: "Requesting a boolean field path should work.",
			path:   "metadata.ownerRefs[0].controller",
			data:   []byte(`{"metadata":{"ownerRefs":[{"controller": true}]}}`),
			want: want{
				value: true,
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotABool": {
			reason: "Requesting an non-boolean field path should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				err: errors.New("metadata.name: not a bool"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetBool(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetBool(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetBool(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestGetNumber(t *testing.T) {
	type want struct {
		value float64
		err   error
	}
	cases := map[string]struct {
		reason string
		path   string
		data   []byte
		want   want
	}{
		"MetadataVersion": {
			reason: "Requesting a number field should work",
			path:   "metadata.version",
			data:   []byte(`{"metadata":{"version":2}}`),
			want: want{
				value: 2,
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			path:   "spec[]",
			want: want{
				err: errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
		"NotANumber": {
			reason: "Requesting an non-number field path should fail",
			path:   "metadata.name",
			data:   []byte(`{"metadata":{"name":"cool"}}`),
			want: want{
				err: errors.New("metadata.name: not a (float64) number"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			got, err := p.GetNumber(tc.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.GetNumber(%s): %s: -want error, +got error:\n%s", tc.path, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("\np.GetNumber(%s): %s: -want, +got:\n%s", tc.path, tc.reason, diff)
			}
		})
	}
}

func TestSetValue(t *testing.T) {
	type args struct {
		path  string
		value interface{}
	}
	type want struct {
		object map[string]interface{}
		err    error
	}
	cases := map[string]struct {
		reason string
		data   []byte
		args   args
		want   want
	}{
		"MetadataName": {
			reason: "Setting an object field should work",
			data:   []byte(`{"metadata":{"name":"lame"}}`),
			args: args{
				path:  "metadata.name",
				value: "cool",
			},
			want: want{
				object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "cool",
					},
				},
			},
		},
		"NonExistentMetadataName": {
			reason: "Setting a non-existent object field should work",
			data:   []byte(`{}`),
			args: args{
				path:  "metadata.name",
				value: "cool",
			},
			want: want{
				object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "cool",
					},
				},
			},
		},
		"ContainerName": {
			reason: "Setting a field of an object that is an array element should work",
			data:   []byte(`{"spec":{"containers":[{"name":"lame"}]}}`),
			args: args{
				path:  "spec.containers[0].name",
				value: "cool",
			},
			want: want{
				object: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "cool",
							},
						},
					},
				},
			},
		},
		"NonExistentContainerName": {
			reason: "Setting a field of a non-existent object that is an array element should work",
			data:   []byte(`{}`),
			args: args{
				path:  "spec.containers[0].name",
				value: "cool",
			},
			want: want{
				object: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "cool",
							},
						},
					},
				},
			},
		},
		"NewContainer": {
			reason: "Growing an array object field should work",
			data:   []byte(`{"spec":{"containers":[{"name":"cool"}]}}`),
			args: args{
				path:  "spec.containers[1].name",
				value: "cooler",
			},
			want: want{
				object: map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name": "cool",
							},
							map[string]interface{}{
								"name": "cooler",
							},
						},
					},
				},
			},
		},
		"NestedArray": {
			reason: "Setting a value in a nested array should work",
			data:   []byte(`{}`),
			args: args{
				path:  "data[0][0]",
				value: "a",
			},
			want: want{
				object: map[string]interface{}{
					"data": []interface{}{
						[]interface{}{"a"},
					},
				},
			},
		},
		"GrowNestedArray": {
			reason: "Growing then setting a value in a nested array should work",
			data:   []byte(`{"data":[["a"]]}`),
			args: args{
				path:  "data[0][1]",
				value: "b",
			},
			want: want{
				object: map[string]interface{}{
					"data": []interface{}{
						[]interface{}{"a", "b"},
					},
				},
			},
		},
		"GrowArrayField": {
			reason: "Growing then setting a value in an array field should work",
			data:   []byte(`{"data":["a"]}`),
			args: args{
				path:  "data[2]",
				value: "c",
			},
			want: want{
				object: map[string]interface{}{
					"data": []interface{}{"a", nil, "c"},
				},
			},
		},
		"MapStringString": {
			reason: "A map of string to string should be converted to a map of string to interface{}",
			data:   []byte(`{"metadata":{}}`),
			args: args{
				path:  "metadata.labels",
				value: map[string]string{"cool": "very"},
			},
			want: want{
				object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{"cool": "very"},
					},
				},
			},
		},
		"OwnerReference": {
			reason: "An ObjectReference (i.e. struct) should be converted to a map of string to interface{}",
			data:   []byte(`{"metadata":{}}`),
			args: args{
				path: "metadata.ownerRefs[0]",
				value: metav1.OwnerReference{
					APIVersion: "v",
					Kind:       "k",
					Name:       "n",
					UID:        types.UID("u"),
				},
			},
			want: want{
				object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"ownerRefs": []interface{}{
							map[string]interface{}{
								"apiVersion": "v",
								"kind":       "k",
								"name":       "n",
								"uid":        "u",
							},
						},
					},
				},
			},
		},
		"NotAnArray": {
			reason: "Indexing an object field should fail",
			data:   []byte(`{"data":{}}`),
			args: args{
				path: "data[0]",
			},
			want: want{
				object: map[string]interface{}{"data": map[string]interface{}{}},
				err:    errors.New("data is not an array"),
			},
		},
		"NotAnObject": {
			reason: "Requesting a field in an array should fail",
			data:   []byte(`{"data":[]}`),
			args: args{
				path: "data.name",
			},
			want: want{
				object: map[string]interface{}{"data": []interface{}{}},
				err:    errors.New("data is not an object"),
			},
		},
		"MalformedPath": {
			reason: "Requesting an invalid field path should fail",
			args: args{
				path: "spec[]",
			},
			want: want{
				object: map[string]interface{}{},
				err:    errors.Wrap(errors.New("unexpected ']' at position 5"), "cannot parse path \"spec[]\""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := make(map[string]interface{})
			_ = json.Unmarshal(tc.data, &in)
			p := Pave(in)

			err := p.SetValue(tc.args.path, tc.args.value)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.SetValue(%s, %v): %s: -want error, +got error:\n%s", tc.args.path, tc.args.value, tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.object, p.object); diff != "" {
				t.Fatalf("\np.SetValue(%s, %v): %s: -want, +got:\n%s", tc.args.path, tc.args.value, tc.reason, diff)
			}
		})
	}
}
