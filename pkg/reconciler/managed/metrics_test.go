/*
Copyright 2026 The Crossplane Authors.

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
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
)

func TestMRMetricRecorderNamespacedResources(t *testing.T) {
	// Two managed resources with the same name in different namespaces.
	newMR := func(namespace string) *fake.Managed {
		return &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: "collide", Namespace: namespace}}
	}

	ready := func(mg *fake.Managed) *fake.Managed {
		mg.SetConditions(xpv2.Available())
		return mg
	}

	cases := map[string]struct {
		reason string
		record func(r *MRMetricRecorder)
		want   map[string]uint64
	}{
		"FirstTimeReady": {
			reason: "Each managed resource that becomes ready should be observed, even if another namespace has a managed resource with the same name.",
			record: func(r *MRMetricRecorder) {
				r.recordFirstTimeReconciled(newMR("a"))
				r.recordFirstTimeReconciled(newMR("b"))
				r.recordFirstTimeReady(ready(newMR("a")))
				r.recordFirstTimeReady(ready(newMR("b")))
			},
			want: map[string]uint64{
				"crossplane_managed_resource_first_time_to_readiness_seconds": 2,
			},
		},
		"DriftWithoutPreviousObservation": {
			reason: "Drift should not be observed for a managed resource that was never observed unchanged, even if another namespace has a managed resource with the same name.",
			record: func(r *MRMetricRecorder) {
				r.recordUnchanged(newMR("a"))
				r.recordDrift(newMR("b"))
			},
			want: map[string]uint64{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewMRMetricRecorder()
			tc.record(r)

			reg := prometheus.NewRegistry()
			reg.MustRegister(r)

			mfs, err := reg.Gather()
			if err != nil {
				t.Fatalf("reg.Gather(): %v", err)
			}

			got := map[string]uint64{}

			for _, mf := range mfs {
				if mf.GetName() == "crossplane_managed_resource_first_time_to_reconcile_seconds" {
					continue
				}

				for _, m := range mf.GetMetric() {
					got[mf.GetName()] += m.GetHistogram().GetSampleCount()
				}
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nReason: %s\nMRMetricRecorder: -want sample counts, +got sample counts:\n%s", tc.reason, diff)
			}
		})
	}
}
