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

package statemetrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// A XRStateRecorder records the state of composite resources.
type XRStateRecorder struct {
	client   client.Client
	log      logging.Logger
	interval time.Duration

	compositeExists        *prometheus.GaugeVec
	compositeReady         *prometheus.GaugeVec
	compositeSynced        *prometheus.GaugeVec
	compositeComposedCount *prometheus.GaugeVec
}

// A APIExtStateRecorderOption configures a MRStateRecorder.
type APIExtStateRecorderOption func(*XRStateRecorder)

// NewXRStateRecorder returns a new XRStateRecorder which records the state of claim,
// composite and composition metrics.
func NewXRStateRecorder(client client.Client, log logging.Logger, interval time.Duration) *XRStateRecorder {
	return &XRStateRecorder{
		client:   client,
		log:      log,
		interval: interval,

		compositeExists: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_exists",
			Help:      "The number of composite resources that exist",
		}, []string{"gvk", "composition"}),
		compositeReady: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_ready",
			Help:      "The number of composite resources in Ready=True state",
		}, []string{"gvk", "composition"}),
		compositeSynced: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_synced",
			Help:      "The number of composite resources in Synced=True state",
		}, []string{"gvk", "composition"}),
		compositeComposedCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_composed_count",
			Help:      "The number of composed resources in total",
		}, []string{"gvk", "composition"}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (r *XRStateRecorder) Describe(ch chan<- *prometheus.Desc) {
	r.compositeExists.Describe(ch)
	r.compositeReady.Describe(ch)
	r.compositeSynced.Describe(ch)
	r.compositeComposedCount.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (r *XRStateRecorder) Collect(ch chan<- prometheus.Metric) {
	r.compositeExists.Collect(ch)
	r.compositeReady.Collect(ch)
	r.compositeSynced.Collect(ch)
	r.compositeComposedCount.Collect(ch)
}

// Record records the state of managed resources.
func (r *XRStateRecorder) Record(ctx context.Context, gvk schema.GroupVersionKind) {
	xrs := &unstructured.UnstructuredList{}
	xrs.SetGroupVersionKind(gvk)
	err := r.client.List(ctx, xrs)
	if err != nil {
		r.log.Info("Failed to list composite resources", "error", err)
		return
	}

	composition, err := getCompositionRef(xrs)
	if err != nil {
		r.log.Info("Failed to get composition reference of composite resource", "error", err)
		return
	}

	labels := prometheus.Labels{
		"gvk":         gvk.String(),
		"composition": composition,
	}
	r.compositeExists.With(labels).Set(float64(len(xrs.Items)))

	var numReady, numSynced, numComposed float64 = 0, 0, 0
	for _, xr := range xrs.Items {
		conditioned := xpv1.ConditionedStatus{}
		if err := fieldpath.Pave(xr.Object).GetValueInto("status", &conditioned); err != nil {
			r.log.Info("Failed to get conditions of managed resource", "error", err)
			continue
		}

		for _, condition := range conditioned.Conditions {
			if condition.Type == xpv1.TypeReady && condition.Status == corev1.ConditionTrue {
				numReady++
			} else if condition.Type == xpv1.TypeSynced && condition.Status == corev1.ConditionTrue {
				numSynced++
			}
		}

		resourceRefs := make([]v1.ObjectReference, 0)
		if err := fieldpath.Pave(xr.Object).GetValueInto("spec.resourceRefs", &resourceRefs); err != nil {
			r.log.Info("Failed to get resource references of composed resource", "error", err)
			continue
		}

		numComposed += float64(len(resourceRefs))
	}

	r.compositeReady.With(labels).Set(numReady)
	r.compositeSynced.With(labels).Set(numSynced)
	r.compositeComposedCount.With(labels).Set(numComposed)
}

// Run records state of managed resources with given interval.
func (r *XRStateRecorder) Run(ctx context.Context, gvk schema.GroupVersionKind) {
	ticker := time.NewTicker(r.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				r.Record(ctx, gvk)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func getCompositionRef(l *unstructured.UnstructuredList) (string, error) {
	if len(l.Items) == 0 {
		return "", nil
	}

	xr := l.Items[0].Object
	compRef, err := fieldpath.Pave(xr).GetString("spec.compositionRef")
	if err != nil {
		return "", err
	}

	return compRef, nil
}
