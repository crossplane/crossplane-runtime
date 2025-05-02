/*
Copyright 2025 The Crossplane Authors.

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

package conditions

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		want Manager
	}{{
		name: "New returns a non-nil manager.",
		want: &managerImpl{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_conditionSet_Mark(t *testing.T) {
	manager := New()

	tests := []struct {
		name  string
		start []xpv1.Condition
		mark  []xpv1.Condition
		want  []xpv1.Condition
	}{{
		name:  "provide no conditions",
		start: nil,
		mark:  nil,
		want:  nil,
	}, {
		name:  "empty status, a new condition is appended",
		start: nil,
		mark:  []xpv1.Condition{xpv1.ReconcileSuccess()},
		want:  []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(42)},
	}, {
		name:  "existing status, attempt to mark nothing",
		start: []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
		mark:  nil,
		want:  []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
	}, {
		name:  "existing status, an existing condition is updated",
		start: []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(1)},
		mark:  []xpv1.Condition{xpv1.ReconcileSuccess()},
		want:  []xpv1.Condition{xpv1.ReconcileSuccess().WithObservedGeneration(42)},
	}, {
		name:  "existing status, a new condition is appended",
		start: []xpv1.Condition{xpv1.Available().WithObservedGeneration(1)},
		mark:  []xpv1.Condition{xpv1.ReconcileSuccess()},
		want:  []xpv1.Condition{xpv1.Available().WithObservedGeneration(1), xpv1.ReconcileSuccess().WithObservedGeneration(42)},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ut := newManaged(42, tt.start...)
			c := manager.For(ut)
			c.Mark(tt.mark...)
			if diff := cmp.Diff(tt.want, ut.Conditions, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
				reason := "Failed to update conditions."
				t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
			}
		})
	}

	t.Run("Manage a nil object", func(t *testing.T) {
		c := manager.For(nil)
		if c == nil {
			t.Errorf("manager.For(nil) = %v, want non-nil", c)
		}
		// Test that Marking on a Manager that has a nil object does not end up panicking.
		c.Mark(xpv1.ReconcileSuccess())
		// Success!
	})
}

func Test_managerImpl_For(t *testing.T) {
	tests := []struct {
		name string
		o    ObjectWithConditions
		want ConditionSet
	}{{
		name: "Nil object returns a non-nil manager.",
		want: &conditionSet{},
	}, {
		name: "Object propagates into manager.",
		o:    &fake.Managed{},
		want: &conditionSet{
			o: &fake.Managed{},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := managerImpl{}
			if got := m.For(tt.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("For() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newManaged(generation int64, conditions ...xpv1.Condition) *fake.Managed {
	mg := &fake.Managed{}
	mg.Generation = generation
	mg.SetConditions(conditions...)
	return mg
}
