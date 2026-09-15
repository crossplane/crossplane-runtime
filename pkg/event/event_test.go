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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type EventWrapperFunction func(reason, message string, annotations map[string]string)

type testEventRecorder struct {
	EventWrapperFunction
}

func (r *testEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.EventWrapperFunction(reason, message, nil)
}
func (r *testEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	r.EventWrapperFunction(reason, fmt.Sprintf(messageFmt, args...), nil)
}
func (r *testEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	r.EventWrapperFunction(reason, fmt.Sprintf(messageFmt, args...), annotations)
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

func TestEvent(t *testing.T) {
	var currentEvent Event

	allowedNamespace := "allowed-namespace"
	forbiddenNamespace := "forbidden-namespace"

	basicInputEvent := Event{
		Type:        TypeNormal,
		Reason:      "Basic reason",
		Message:     "Basic message",
		Annotations: map[string]string{},
	}

	emptyEvent := Event{
		Type:        "",
		Reason:      "",
		Message:     "",
		Annotations: nil,
	}

	inputCMInAllowedNamespace := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: allowedNamespace,
		},
	}

	inputCMInForbiddenNamespace := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: forbiddenNamespace,
		},
	}

	testRecorder := &testEventRecorder{
		EventWrapperFunction: func(reason, message string, annotations map[string]string) {
			currentEvent = Event{
				Type:        TypeNormal,
				Reason:      Reason(reason),
				Message:     message,
				Annotations: annotations,
			}
		},
	}

	cases := map[string]struct {
		reason          string
		filterFunctions []FilterFn
		inputResource   runtime.Object
		inputEvent      Event
		want            Event
		annotations     []string
	}{
		"NoFilter": {
			reason:     "Events should always be emitted when there is no filter function.",
			inputEvent: basicInputEvent,
			want:       basicInputEvent,
		},
		"AllowAllFilter": {
			reason: "Events should always be emitted when the filter function allows all events.",
			filterFunctions: []FilterFn{
				func(_ runtime.Object, _ Event) bool {
					return false
				},
			},
			inputEvent: basicInputEvent,
			want:       basicInputEvent,
		},
		"DenyAllFilter": {
			reason: "Events should never be emitted when the filter function denies all events.",
			filterFunctions: []FilterFn{
				func(_ runtime.Object, _ Event) bool {
					return true
				},
			},
			inputEvent: basicInputEvent,
			want:       emptyEvent,
		},
		"NamespaceFilterDeny": {
			reason: "Events should be denied when the filter function denies events from a specific namespace.",
			filterFunctions: []FilterFn{
				func(o runtime.Object, _ Event) bool {
					meta, err := meta.Accessor(o)
					if err != nil {
						return false
					}
					return meta.GetNamespace() == forbiddenNamespace
				},
			},
			inputResource: &inputCMInForbiddenNamespace,
			inputEvent:    basicInputEvent,
			want:          emptyEvent,
		},
		"NamespaceFilterAllow": {
			reason: "Events should be emitted when the filter function allows events from all namespaces but a specific one, and the resource is not in that namespace.",
			filterFunctions: []FilterFn{
				func(o runtime.Object, _ Event) bool {
					meta, err := meta.Accessor(o)
					if err != nil {
						return false
					}
					return meta.GetNamespace() == forbiddenNamespace
				},
			},
			inputResource: &inputCMInAllowedNamespace,
			inputEvent:    basicInputEvent,
			want:          basicInputEvent,
		},
		"WithAnnotations": {
			reason: "Events should be emitted with annotations when WithAnnotations is used.",
			annotations: []string{
				"testKey", "testValue",
			},
			inputEvent: basicInputEvent,
			want: Event{
				Type:    TypeNormal,
				Reason:  basicInputEvent.Reason,
				Message: basicInputEvent.Message,
				Annotations: map[string]string{
					"testKey": "testValue",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Test with NewAPIRecorder
			// AND WithAnnotations to check if it correctly preserves the filter functions, and also to test annotations
			apiRecorder := NewAPIRecorder(testRecorder, tc.filterFunctions...).WithAnnotations(tc.annotations...)
			apiRecorder.Event(tc.inputResource, tc.inputEvent)

			if diff := cmp.Diff(tc.want, currentEvent); diff != "" {
				t.Errorf("%s\nEvent(...): -want, +got:\n%s", tc.reason, diff)
			}
		})

		currentEvent = Event{}
	}
}
