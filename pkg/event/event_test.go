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

package event

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mockKubeRecorder satisfies events.EventRecorder.
type mockKubeRecorder struct {
	events []mockEvent
}

type mockEvent struct {
	obj     runtime.Object
	typeStr string
	reason  string
	msg     string
}

func (m *mockKubeRecorder) Eventf(obj runtime.Object, _ runtime.Object, eventtype, reason, _, note string, args ...any) {
	msg := fmt.Sprintf(note, args...)
	m.events = append(m.events, mockEvent{obj: obj, typeStr: eventtype, reason: reason, msg: msg})
}

type mockObj struct{}

func (m *mockObj) GetObjectKind() schema.ObjectKind { return nil }
func (m *mockObj) DeepCopyObject() runtime.Object {
	return &mockObj{}
}

func TestSliceMap(t *testing.T) {
	type args struct {
		from []string
		to   map[string]string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   map[string]string
	}{
		"OnePair": {
			reason: "One key value pair should be added.",
			args: args{
				from: []string{"key", "val"},
				to:   map[string]string{},
			},
			want: map[string]string{"key": "val"},
		},
		"TwoPairs": {
			reason: "Two key value pairs should be added.",
			args: args{
				from: []string{
					"key", "val",
					"another", "value",
				},
				to: map[string]string{},
			},
			want: map[string]string{
				"key":     "val",
				"another": "value",
			},
		},
		"NoValue": {
			reason: "Two key value pairs should be added.",
			args: args{
				from: []string{"key"},
				to:   map[string]string{},
			},
			want: map[string]string{},
		},
		"ExtraneousKey": {
			reason: "One key value pair should be added.",
			args: args{
				from: []string{
					"key", "val",
					"extraneous",
				},
				to: map[string]string{},
			},
			want: map[string]string{"key": "val"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sliceMap(tc.args.from, tc.args.to)

			if diff := cmp.Diff(tc.want, tc.args.to); diff != "" {
				t.Errorf("%s\nsliceMap(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestAPIRecorderEvent(t *testing.T) {
	mr := &mockKubeRecorder{}
	rec := NewAPIRecorder(mr)

	rec.Event(&mockObj{}, Normal("testReason", "test message"))

	want := mockEvent{typeStr: "Normal", reason: "testReason", msg: "test message"}
	if diff := cmp.Diff(want, mr.events[0], cmp.AllowUnexported(mockEvent{}), cmpopts.IgnoreFields(mockEvent{}, "obj")); diff != "" {
		t.Errorf("unexpected event: -want, +got:\n%s", diff)
	}
}

func TestAPIRecorderFilter(t *testing.T) {
	mr := &mockKubeRecorder{}
	filter := func(_ runtime.Object, _ Event) bool { return true }
	rec := NewAPIRecorder(mr, filter)

	rec.Event(&mockObj{}, Normal("testReason", "test message"))

	if diff := cmp.Diff(0, len(mr.events)); diff != "" {
		t.Errorf("expected no events, got %d: %s", len(mr.events), diff)
	}
}

func TestAPIRecorderWithAnnotationsPreservesFilterFns(t *testing.T) {
	filterCalled := false
	filter := func(_ runtime.Object, _ Event) bool {
		filterCalled = true
		return false
	}

	mr := &mockKubeRecorder{}
	rec := NewAPIRecorder(mr, filter)
	derived := rec.WithAnnotations("key", "val")

	derived.Event(&mockObj{}, Normal("test", "msg"))

	if diff := cmp.Diff(true, filterCalled); diff != "" {
		t.Errorf("filter function was not preserved after WithAnnotations: %s", diff)
	}
}

func TestAPIRecorderWithAnnotationsPreservesExistingAnnotations(t *testing.T) {
	mr := &mockKubeRecorder{}
	rec := NewAPIRecorder(mr)
	r1 := rec.WithAnnotations("k1", "v1").(*APIRecorder)
	r2 := r1.WithAnnotations("k2", "v2").(*APIRecorder)

	want := map[string]string{"k1": "v1", "k2": "v2"}
	if diff := cmp.Diff(want, r2.annotations); diff != "" {
		t.Errorf("annotations not preserved correctly: -want, +got:\n%s", diff)
	}
}
