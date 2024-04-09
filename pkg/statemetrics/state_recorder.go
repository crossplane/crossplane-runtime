/*
Copyright 2024 The Crossplane Authors.

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

// Package statemetrics contains utilities for recording Crossplane resource state metrics.
package statemetrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const subSystem = "crossplane"

// A StateRecorder records the state of given GroupVersionKind.
type StateRecorder interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)

	Record(ctx context.Context, gvk schema.GroupVersionKind)
	Run(ctx context.Context, gvk schema.GroupVersionKind)
}

// A NopStateRecorder does nothing.
type NopStateRecorder struct{}

// NewNopStateRecorder returns a NopStateRecorder that does nothing.
func NewNopStateRecorder() *NopStateRecorder {
	return &NopStateRecorder{}
}

// Describe does nothing.
func (r *NopStateRecorder) Describe(_ chan<- *prometheus.Desc) {}

// Collect does nothing.
func (r *NopStateRecorder) Collect(_ chan<- prometheus.Metric) {}

// Record does nothing.
func (r *NopStateRecorder) Record(_ context.Context, _ schema.GroupVersionKind) {}

// Run does nothing.
func (r *NopStateRecorder) Run(_ context.Context, _ schema.GroupVersionKind) {}
