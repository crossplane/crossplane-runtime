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

// Package gate contains a gated function callback registration implementation.
package gate

import (
	"slices"
	"sync"
)

// Gate implements a gated function callback registration with comparable conditions.
type Gate[T comparable] struct {
	mux        sync.RWMutex
	conditions map[T]bool
	fns        []gated[T]
}

type gated[T comparable] struct {
	fn       func()
	depends  []T
	released bool
}

// Register a callback function that will be called when all the provided dependent conditions are true.
// After all conditions are true, the callback function is removed from the registration and will not be called again.
// Thread Safe.
func (g *Gate[T]) Register(fn func(), depends ...T) {
	g.mux.Lock()
	g.fns = append(g.fns, gated[T]{fn: fn, depends: depends})
	g.mux.Unlock()

	g.process()
}

// True marks the associated condition as true. If the condition is already true, then this is a no-op.
// Returns if there was an update detected. Thread safe.
func (g *Gate[T]) True(condition T) bool {
	return g.markCondition(condition, true)
}

// False marks the associated condition as false. If the condition is already false, then this is a no-op.
// Returns if there was an update detected. Thread safe.
func (g *Gate[T]) False(condition T) bool {
	return g.markCondition(condition, false)
}

func (g *Gate[T]) markCondition(condition T, ready bool) bool {
	g.mux.Lock()

	if g.conditions == nil {
		g.conditions = make(map[T]bool)
	}

	old, found := g.conditions[condition]

	updated := false
	if !found || old != ready {
		updated = true
		g.conditions[condition] = ready
	}
	// process() would also like to lock the mux, so we must unlock here directly and not use defer.
	g.mux.Unlock()

	if updated {
		g.process()
	}

	return updated
}

func (g *Gate[T]) process() {
	g.mux.Lock()
	defer g.mux.Unlock()

	for i := range g.fns {
		release := true

		for _, dep := range g.fns[i].depends {
			if ready := g.conditions[dep]; !ready {
				release = false
			}
		}

		if release {
			fn := g.fns[i].fn
			g.fns[i].released = true
			// Need to capture a copy of fn or else we would be accessing a deleted member when the go routine runs.
			go fn()
		}
	}

	g.fns = slices.DeleteFunc(g.fns, func(a gated[T]) bool {
		return a.released
	})
}
