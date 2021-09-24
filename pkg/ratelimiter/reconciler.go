/*
Copyright 2021 The Crossplane Authors.

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

package ratelimiter

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// A Reconciler rate limits an inner, wrapped Reconciler. Requests that are rate
// limited immediately return RequeueAfter: d without calling the wrapped
// Reconciler, where d is imposed by the rate limiter.
type Reconciler struct {
	name  string
	inner reconcile.Reconciler
	limit ratelimiter.RateLimiter
}

// NewReconciler wraps the supplied Reconciler, ensuring requests are passed to
// it no more frequently than the supplied RateLimiter allows. Multiple uniquely
// named Reconcilers can share the same RateLimiter.
func NewReconciler(name string, r reconcile.Reconciler, l ratelimiter.RateLimiter) *Reconciler {
	return &Reconciler{name: name, inner: r, limit: l}
}

// Reconcile the supplied request subject to rate limiting.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	item := r.name + req.String()
	if d := r.limit.When(item); d > time.Duration(0) {
		return reconcile.Result{RequeueAfter: d}, nil
	}
	r.limit.Forget(item)
	return r.inner.Reconcile(ctx, req)
}
