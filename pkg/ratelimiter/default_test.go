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
	"testing"
	"time"
)

func TestDefaultManagedRateLimiter(t *testing.T) {
	limiter := NewDefaultManagedRateLimiter(NewDefaultProviderRateLimiter(DefaultProviderRPS))
	backoffSchedule := []int{1, 2, 4, 8, 16, 32, 60}
	for _, d := range backoffSchedule {
		if e, a := time.Duration(d)*time.Second, limiter.When("one"); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}
	}
	limiter.Forget("one")
	if e, a := time.Duration(backoffSchedule[0])*time.Second, limiter.When("one"); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}
