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

// Package conditions enables consistent interactions with an object's status conditions.
package conditions

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// ObjectWithConditions is the interface definition that allows.
type ObjectWithConditions interface {
	resource.Object
	resource.Conditioned
}

// Manager is an interface for a stateless factory-like object that produces ConditionSet objects.
type Manager interface {
	// For returns an implementation of a ConditionSet to operate on a specific ObjectWithConditions.
	For(o ObjectWithConditions) ConditionSet
}

// ConditionSet holds operations for interacting with an object's conditions.
type ConditionSet interface {
	// Mark adds or updates the conditions onto the managed resource object. Unlike a "Set" method, this also can add
	// contextual updates to the condition such as propagating the correct observedGeneration to the conditions being
	// changed.
	Mark(condition ...xpv1.Condition)
}

// New returns an implementation of a Manager.
func New() Manager {
	return &managerImpl{}
}

// Check that conditionsImpl implements ConditionManager.
var _ Manager = (*managerImpl)(nil)

// managerImpl is the top level factor for producing a ConditionSet on behalf of a ObjectWithConditions resource.
// managerImpl implements Manager.
type managerImpl struct{}

// For implements Manager.For.
func (m managerImpl) For(o ObjectWithConditions) ConditionSet {
	return &conditionSet{o: o}
}

// Check that conditionSet implements ConditionSet.
var _ ConditionSet = (*conditionSet)(nil)

type conditionSet struct {
	o ObjectWithConditions
}

// Mark implements ConditionSet.Mark.
func (c *conditionSet) Mark(condition ...xpv1.Condition) {
	if c == nil || c.o == nil {
		return
	}
	// Foreach condition we have been sent to mark, update the observed generation.
	for i := range condition {
		condition[i].ObservedGeneration = c.o.GetGeneration()
	}
	c.o.SetConditions(condition...)
}
