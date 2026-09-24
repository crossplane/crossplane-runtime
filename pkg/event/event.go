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

// Package event records Kubernetes events.
package event

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
)

// A Type of event.
type Type string

// Event types.
// https://pkg.go.dev/k8s.io/client-go/tools/events#EventRecorder
const (
	TypeNormal  Type = "Normal"
	TypeWarning Type = "Warning"
)

// Reason an event occurred.
type Reason string

// An Event relating to a Crossplane resource.
type Event struct {
	Type    Type
	Reason  Reason
	Message string
}

// Normal returns a normal, informational event.
func Normal(r Reason, message string) Event {
	return Event{
		Type:    TypeNormal,
		Reason:  r,
		Message: message,
	}
}

// Warning returns a warning event, typically due to an error.
func Warning(r Reason, err error) Event {
	return Event{
		Type:    TypeWarning,
		Reason:  r,
		Message: err.Error(),
	}
}

// A Recorder records Kubernetes events.
type Recorder interface {
	Event(obj runtime.Object, e Event)
}

// FilterFn is a function used to filter events. Returning true prevents the
// event from being recorded.
type FilterFn func(obj runtime.Object, e Event) bool

// An APIRecorder records Kubernetes events to an API server using the
// events.k8s.io/v1 API introduced in Kubernetes 1.19.
//
// Note: the events.k8s.io API does not support per-event annotations (unlike
// the deprecated record.EventRecorder.AnnotatedEventf). Callers that previously
// relied on annotation propagation should encode that metadata into the event
// message instead.
type APIRecorder struct {
	kube      events.EventRecorder
	filterFns []FilterFn
}

// NewAPIRecorder returns an APIRecorder that records Kubernetes events to an
// API server using the supplied EventRecorder.
func NewAPIRecorder(r events.EventRecorder, fns ...FilterFn) *APIRecorder {
	return &APIRecorder{kube: r, filterFns: fns}
}

// Event records the supplied event.
func (r *APIRecorder) Event(obj runtime.Object, e Event) {
	for _, filter := range r.filterFns {
		if filter(obj, e) {
			return
		}
	}

	r.kube.Eventf(obj, nil, string(e.Type), string(e.Reason), string(e.Reason), "%s", e.Message)
}

// A NopRecorder does nothing.
type NopRecorder struct{}

// NewNopRecorder returns a Recorder that does nothing.
func NewNopRecorder() *NopRecorder {
	return &NopRecorder{}
}

// Event does nothing.
func (r *NopRecorder) Event(_ runtime.Object, _ Event) {}
