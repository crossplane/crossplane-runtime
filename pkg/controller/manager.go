/*
Copyright 2020 The Crossplane Authors.

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

package controller

import (
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TODO potentially this can be extracted from _some_
// configuration (i.e. crossplane itself and providers).
// New attributes can be added to set enable/disable this
// and also the number of desired replicas, with some sane
// default for production deployment. This can be useful to
// opt out of this 1) on development 2) CI tests 3) per user
// decision.
var leaderElection = true

// NewManager returns a new Manager for creating Controllers.
// This takes crossplane specific configuration into consideration
// to either enable leader election for the said Manager or opt
// out of it.
func NewManager(config *rest.Config, options ctrl.Options) (ctrl.Manager, error) {
	if leaderElection {
		options.LeaderElection = true
		options.LeaderElectionID = "crossplane-leader-election-id"
		options.LeaderElectionNamespace = "crossplane-system"
	}
	return ctrl.NewManager(config, options)
}
