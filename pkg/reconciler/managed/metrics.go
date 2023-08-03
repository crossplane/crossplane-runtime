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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	kmetrics "k8s.io/component-base/metrics"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

func init() {
	metrics.Registry.MustRegister(drift)
}

var subSystem = "crossplane"

var (
	drift = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: subSystem,
		Name:      "resource_drift_seconds",
		Help:      "ALPHA: How long since the previous successful reconcile when a resource was found to be out of sync; excludes restart of the provider",
		Buckets:   kmetrics.ExponentialBuckets(10e-9, 10, 10),
	}, []string{"group", "kind"})
)

// driftRecorder records the time since the last observation of a resource
// and records the time since on update as a metric. This represents an upper
// bound for the duration the drift existed.
type driftRecorder struct {
	lastObservation sync.Map
	gvk             schema.GroupVersionKind

	cluster cluster.Cluster
}

var _ manager.Runnable = &driftRecorder{}

func (r *driftRecorder) Start(ctx context.Context) error {
	inf, err := r.cluster.GetCache().GetInformerForKind(ctx, r.gvk)
	if err != nil {
		return errors.Wrapf(err, "cannot get informer for drift recorder for resource %s", r.gvk)
	}

	registered, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			if final, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = final.Obj
			}
			managed := obj.(resource.Managed)
			r.lastObservation.Delete(managed.GetName())
		},
	})
	if err != nil {
		return errors.Wrap(err, "cannot add delete event handler to informer for drift recorder")
	}
	defer inf.RemoveEventHandler(registered) //nolint:errcheck // this happens on destruction. We cannot do anything anyway.

	<-ctx.Done()

	return nil
}

func (r *driftRecorder) recordUnchanged(name string) {
	r.lastObservation.Store(name, time.Now())
}

func (r *driftRecorder) recordUpdate(name string) {
	last, ok := r.lastObservation.Load(name)
	if !ok {
		return
	}
	drift.WithLabelValues(r.gvk.Group, r.gvk.Kind).Observe(time.Since(last.(time.Time)).Seconds())

	r.lastObservation.Store(name, time.Now())
}
