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
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// A XRStateMetrics holds Prometheus metrics for composite resources.
type XRStateMetrics struct {
	Exists        *prometheus.GaugeVec
	Ready         *prometheus.GaugeVec
	Synced        *prometheus.GaugeVec
	ComposedCount *prometheus.GaugeVec
}

// NewXRStateMetrics returns a new XRStateMetrics.
func NewXRStateMetrics() *XRStateMetrics {
	return &XRStateMetrics{
		Exists: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_exists",
			Help:      "The number of composite resources that exist",
		}, []string{"gvk", "composition"}),
		Ready: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_ready",
			Help:      "The number of composite resources in Ready=True state",
		}, []string{"gvk", "composition"}),
		Synced: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_synced",
			Help:      "The number of composite resources in Synced=True state",
		}, []string{"gvk", "composition"}),
		ComposedCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Subsystem: subSystem,
			Name:      "composite_resource_composed_count",
			Help:      "The number of composed resources in total",
		}, []string{"gvk", "composition"}),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (r *XRStateMetrics) Describe(ch chan<- *prometheus.Desc) {
	r.Exists.Describe(ch)
	r.Ready.Describe(ch)
	r.Synced.Describe(ch)
	r.ComposedCount.Describe(ch)
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (r *XRStateMetrics) Collect(ch chan<- prometheus.Metric) {
	r.Exists.Collect(ch)
	r.Ready.Collect(ch)
	r.Synced.Collect(ch)
	r.ComposedCount.Collect(ch)
}

// A XRStateRecorder records the state of composite resources.
type XRStateRecorder struct {
	client        client.Client
	log           logging.Logger
	interval      time.Duration
	compositeList resource.CompositeList

	metrics *XRStateMetrics
}

// NewXRStateRecorder returns a new XRStateRecorder which records the state xr resources.
func NewXRStateRecorder(client client.Client, log logging.Logger, metrics *XRStateMetrics, compositeList resource.CompositeList, interval time.Duration) *XRStateRecorder {
	return &XRStateRecorder{
		client:        client,
		log:           log,
		metrics:       metrics,
		compositeList: compositeList,
		interval:      interval,
	}
}

// Record records the state of managed resources.
func (r *XRStateRecorder) Record(ctx context.Context, xrList resource.CompositeList) error {
	if err := r.client.List(ctx, xrList); err != nil {
		r.log.Info("Failed to list composite resources", "error", err)
		return err
	}

	xrs := xrList.GetItems()
	if len(xrs) == 0 {
		return nil
	}

	labels := getLabels(xrs)
	r.metrics.Exists.With(labels).Set(float64(len(xrs)))

	var numReady, numSynced, numComposed float64 = 0, 0, 0
	for _, xr := range xrs {
		if xr.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
			numReady++
		}

		if xr.GetCondition(xpv1.TypeSynced).Status == corev1.ConditionTrue {
			numSynced++
		}

		numComposed += float64(len(xr.GetResourceReferences()))
	}

	r.metrics.Ready.With(labels).Set(numReady)
	r.metrics.Synced.With(labels).Set(numSynced)
	r.metrics.ComposedCount.With(labels).Set(numComposed)

	return nil
}

// Start records state of managed resources with given interval.
func (r *XRStateRecorder) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	for {
		select {
		case <-ticker.C:
			if err := r.Record(ctx, r.compositeList); err != nil {
				return err
			}
		case <-ctx.Done():
			ticker.Stop()
			return nil
		}
	}
}

func getLabels(xrs []resource.Composite) prometheus.Labels {
	xr := xrs[0]
	labels := prometheus.Labels{
		"gvk":         xr.GetObjectKind().GroupVersionKind().String(),
		"composition": xr.GetCompositionReference().String(),
	}

	return labels
}
