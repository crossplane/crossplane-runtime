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
	action  string
	msg     string
}

func (m *mockKubeRecorder) Eventf(obj runtime.Object, _ runtime.Object, eventtype, reason, action, note string, args ...any) {
	msg := fmt.Sprintf(note, args...)
	m.events = append(m.events, mockEvent{obj: obj, typeStr: eventtype, reason: reason, action: action, msg: msg})
}

type mockObj struct{}

func (m *mockObj) GetObjectKind() schema.ObjectKind { return nil }
func (m *mockObj) DeepCopyObject() runtime.Object {
	return &mockObj{}
}

func TestAPIRecorderEvent(t *testing.T) {
	mr := &mockKubeRecorder{}
	rec := NewAPIRecorder(mr)

	rec.Event(&mockObj{}, Normal("testReason", "test message"))

	want := mockEvent{typeStr: "Normal", reason: "testReason", action: "testReason", msg: "test message"}
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
