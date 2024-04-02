/*
Copyright 2023 The Crossplane Authors.

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

package managed

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	kmetrics "k8s.io/component-base/metrics"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

func init() { //nolint:gochecknoinits // metrics should be registered once
	metrics.Registry.MustRegister(drift, mr, mrReady, mrSynced, mrDetected, mrReadyDuration, mrDeletionDuration)
}

const subSystem = "crossplane"

var (
	drift = prometheus.NewHistogramVec(prometheus.HistogramOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "resource_drift_seconds",
		Help:      "ALPHA: How long since the previous successful reconcile when a resource was found to be out of sync; excludes restart of the provider",
		Buckets:   kmetrics.ExponentialBuckets(10e-9, 10, 10),
	}, []string{"group", "kind"})

	mr = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_created",
		Help:      "The number of managed resources created",
	}, []string{"gvk", "name", "claim", "composite"})

	mrReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_ready",
		Help:      "The number of managed resources in Ready=True state",
	}, []string{"gvk", "name", "claim", "composite"})

	mrReadyDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_ready_duration_seconds",
		Help:      "The time it took for a managed resource to become ready first time after creation",
		Buckets:   []float64{1, 5, 10, 15, 30, 60, 120, 300, 600, 1800, 3600},
	}, []string{"gvk", "name", "claim", "composite"})

	mrDetected = prometheus.NewHistogramVec(prometheus.HistogramOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_detected_time_seconds",
		Help:      "The time it took for a managed resource to be detected by the controller",
		Buckets:   kmetrics.ExponentialBuckets(10e-9, 10, 10),
	}, []string{"gvk", "name", "claim", "composite"})

	mrSynced = prometheus.NewGaugeVec(prometheus.GaugeOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_synced",
		Help:      "The number of managed resources in Synced=True state",
	}, []string{"gvk", "name", "claim", "composite"})

	mrDeletionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{ //nolint:gochecknoglobals // metrics should be registered once in init
		Subsystem: subSystem,
		Name:      "managed_resource_deletion_seconds",
		Help:      "The time it took for a managed resource to be deleted",
		Buckets:   []float64{1, 5, 10, 15, 30, 60, 120, 300, 600, 1800, 3600},
	}, []string{"gvk", "name", "claim", "composite"})
)

type metricRecorder struct {
	firstObservation sync.Map
	lastObservation  sync.Map

	cluster cluster.Cluster
	gvk     schema.GroupVersionKind
}

func (r *metricRecorder) Start(ctx context.Context) error {
	inf, err := r.cluster.GetCache().GetInformerForKind(ctx, r.gvk)
	if err != nil {
		return errors.Wrapf(err, "cannot get informer for metric recorder for resource %s", r.gvk)
	}

	registered, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			if final, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = final.Obj
			}
			managed, ok := obj.(resource.Managed)
			if !ok {
				return
			}
			r.firstObservation.Delete(managed.GetName())
			r.lastObservation.Delete(managed.GetName())
		},
	})
	if err != nil {
		return errors.Wrap(err, "cannot add delete event handler to informer for metric recorder")
	}
	defer inf.RemoveEventHandler(registered) //nolint:errcheck // this happens on destruction. We cannot do anything anyway.

	<-ctx.Done()

	return nil
}

func (r *metricRecorder) recordUnchanged(name string) {
	r.lastObservation.Store(name, time.Now())
}

func (r *metricRecorder) recordUpdate(name string) {
	last, ok := r.lastObservation.Load(name)
	if !ok {
		return
	}
	lt, ok := last.(time.Time)
	if !ok {
		return
	}

	drift.WithLabelValues(r.gvk.Group, r.gvk.Kind).Observe(time.Since(lt).Seconds())

	r.lastObservation.Store(name, time.Now())
}

func (r *metricRecorder) recordDetected(managed resource.Managed) {
	if managed.GetCondition(xpv1.TypeSynced).Status == corev1.ConditionUnknown {
		mr.With(getMRMetricLabels(managed)).Set(1)
		mrDetected.With(getMRMetricLabels(managed)).Observe(time.Since(managed.GetCreationTimestamp().Time).Seconds())
		r.firstObservation.Store(managed.GetName(), time.Now()) // this is the first time we reconciled on this resource
	}
}

func (r *metricRecorder) recordSyncedState(managed resource.Managed, v float64) {
	mrSynced.With(getMRMetricLabels(managed)).Set(v)
}

func (r *metricRecorder) recordNotReady(managed resource.Managed) {
	mrReady.With(getMRMetricLabels(managed)).Set(0)
}

func (r *metricRecorder) recordDeleted(managed resource.Managed) {
	labels := getMRMetricLabels(managed)

	if managed.GetDeletionTimestamp() != nil {
		mrDeletionDuration.With(getMRMetricLabels(managed)).Observe(time.Since(managed.GetDeletionTimestamp().Time).Seconds())
	}
	mr.With(labels).Set(0)
	mrReady.With(labels).Set(0)
	mrSynced.With(labels).Set(0)
}

func (r *metricRecorder) recordUpToDate(managed resource.Managed) {
	mrSynced.With(getMRMetricLabels(managed)).Set(1)
	// Note that providers may set the ready condition to "True", so we need
	// to check the value here to send the ready metric
	if managed.GetCondition(xpv1.TypeReady).Status == corev1.ConditionTrue {
		mrReady.With(getMRMetricLabels(managed)).Set(1)
		name := managed.GetName()
		_, ok := r.firstObservation.Load(name) // This map is used to identify the first time to readiness
		if !ok {
			return
		}

		mrReadyDuration.With(getMRMetricLabels(managed)).Observe(time.Since(managed.GetCreationTimestamp().Time).Seconds())
		r.firstObservation.Delete(managed.GetName())
	}
}

func getMRMetricLabels(managed resource.Managed) prometheus.Labels {
	l := prometheus.Labels{
		"gvk":       managed.GetObjectKind().GroupVersionKind().String(),
		"name":      managed.GetName(),
		"claim":     "",
		"composite": managed.GetLabels()["crossplane.io/composite"],
	}

	if managed.GetLabels()["crossplane.io/claim-namespace"] != "" && managed.GetLabels()["crossplane.io/claim-name"] != "" {
		l["claim"] = managed.GetLabels()["crossplane.io/claim-namespace"] + "/" + managed.GetLabels()["crossplane.io/claim-name"]
	}

	return l
}
