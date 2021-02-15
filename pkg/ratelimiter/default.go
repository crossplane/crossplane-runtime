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
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
)

// DefaultProviderRPS is the recommended default average requeues per second
// tolerated by a provider's controller manager.
const DefaultProviderRPS = 1

// NewDefaultProviderRateLimiter returns a token bucket rate limiter meant for
// limiting the number of average total requeues per second for all controllers
// registered with a controller manager. The bucket size is a linear function of
// the requeues per second.
func NewDefaultProviderRateLimiter(rps int) *workqueue.BucketRateLimiter {
	return &workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(rps), rps*10)}
}

// NewDefaultManagedRateLimiter returns a rate limiter that takes the maximum
// delay between the passed providerRateLimiter and a per-item exponential
// backoff limiter. The exponential backoff limiter has a base delay of 1s and a
// maximum of 60s.
func NewDefaultManagedRateLimiter(providerRateLimiter ratelimiter.RateLimiter) ratelimiter.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 60*time.Second),
		providerRateLimiter,
	)
}
