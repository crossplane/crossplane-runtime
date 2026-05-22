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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockRecordRecorder struct {
	events []mockEvent
}

type mockEvent struct {
	obj     runtime.Object
	annots  map[string]string
	typeStr string
	reason  string
	msg     string
}

func (m *mockRecordRecorder) Event(obj runtime.Object, eventtype, reason, message string) {
	m.events = append(m.events, mockEvent{obj: obj, typeStr: eventtype, reason: reason, msg: message})
}

func (m *mockRecordRecorder) Eventf(obj runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
	m.events = append(m.events, mockEvent{obj: obj, typeStr: eventtype, reason: reason, msg: args[0].(string)})
}

func (m *mockRecordRecorder) AnnotatedEventf(obj runtime.Object, annots map[string]string, typeStr, reason, msg string, args ...interface{}) {
	m.events = append(m.events, mockEvent{obj: obj, annots: annots, typeStr: typeStr, reason: reason, msg: args[0].(string)})
}

type mockEventsRecorder struct {
	events []mockEvent
}

func (m *mockEventsRecorder) Eventf(obj runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
	m.events = append(m.events, mockEvent{obj: obj, typeStr: eventtype, reason: reason, msg: args[0].(string)})
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

func TestAPIRecorderWithAnnotationsFilterFns(t *testing.T) {
	filterCalled := false
	filter := func(obj runtime.Object, e Event) bool {
		filterCalled = true
		return false
	}

	mr := &mockRecordRecorder{}
	rec := NewAPIRecorder(mr, filter)
	_ = rec.WithAnnotations("key", "val")

	rec.Event(&mockObj{}, Normal("test", "msg"))

	if !filterCalled {
		t.Error("filter function was not preserved after WithAnnotations")
	}
}

func TestEventsRecorderWithAnnotationsFilterFns(t *testing.T) {
	filterCalled := false
	filter := func(obj runtime.Object, e Event) bool {
		filterCalled = true
		return false
	}

	mr := &mockEventsRecorder{}
	rec := NewEventsRecorder(mr, filter)
	_ = rec.WithAnnotations("key", "val")

	rec.Event(&mockObj{}, Normal("test", "msg"))

	if !filterCalled {
		t.Error("filter function was not preserved after WithAnnotations")
	}
}

func TestEventsRecorderEvent(t *testing.T) {
	mr := &mockEventsRecorder{}
	rec := NewEventsRecorder(mr)

	rec.Event(&mockObj{}, Normal("testReason", "test message"))

	if len(mr.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mr.events))
	}

	if mr.events[0].reason != "testReason" {
		t.Errorf("expected reason 'testReason', got %q", mr.events[0].reason)
	}
	if mr.events[0].msg != "test message" {
		t.Errorf("expected message 'test message', got %q", mr.events[0].msg)
	}
	if mr.events[0].typeStr != "Normal" {
		t.Errorf("expected type 'Normal', got %q", mr.events[0].typeStr)
	}
}

func TestEventsRecorderFilter(t *testing.T) {
	mr := &mockEventsRecorder{}
	filter := func(obj runtime.Object, e Event) bool {
		return true
	}
	rec := NewEventsRecorder(mr, filter)

	rec.Event(&mockObj{}, Normal("testReason", "test message"))

	if len(mr.events) != 0 {
		t.Errorf("expected event to be filtered, got %d events", len(mr.events))
	}
}
