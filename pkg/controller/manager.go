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
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TODO potentially this can be extracted from _some_
// configuration (i.e. crossplane itself and providers).
// New attributes can be added to set enable/disable this
// and also the number of desired replicas, with some sane
// default for production deployment. This can be useful to
// opt out of this 1) on development 2) CI tests 3) per user
// decision.
// var leaderElection = true

// NewManager returns a new Manager for creating Controllers.
// This takes crossplane specific configuration into consideration
// to either enable leader election for the said Manager or opt
// out of it.
func NewManager(config *rest.Config, options Options) (ctrl.Manager, error) {
	ctrloptions := ctrl.Options{
		SyncPeriod:                 options.SyncPeriod,
		LeaderElection:             options.LeaderElection,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
	}

	if options.LeaderElection {
		if options.ManagerName == "" {
			return nil, fmt.Errorf("manager name must be set for leader election")
		}
		ctrloptions.LeaderElectionID = fmt.Sprintf("crossplane-leader-election-%s", options.ManagerName)

		if options.LeaseDuration != nil {
			ctrloptions.LeaseDuration = options.LeaseDuration
		}

		if options.RenewDeadline != nil {
			ctrloptions.RenewDeadline = options.RenewDeadline
		}

		if options.RetryPeriod != nil {
			ctrloptions.RetryPeriod = options.RetryPeriod
		}

		switch {
		case options.LeaderElectionNamespace != "":
			ctrloptions.LeaderElectionNamespace = options.LeaderElectionNamespace
		case options.ManagerNamespace != "":
			ctrloptions.LeaderElectionNamespace = options.ManagerNamespace
		default:
			return nil, fmt.Errorf("TODO: namespace for leader election resources is missing")
		}
	}

	return ctrl.NewManager(config, ctrloptions)
}
