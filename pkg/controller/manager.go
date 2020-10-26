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
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Error strings
const (
	errManagerName      = "manager name is empty"
	errManagerNamespace = "manager namespace is empty"
)

// NewManager returns a new Manager for creating Controllers.
// This takes crossplane specific configuration into consideration
// to either enable leader election for the said Manager or opt
// out of it.
func NewManager(config *rest.Config, options Options) (ctrl.Manager, error) {
	ctrloptions := *options.Options

	if options.LeaderElection {
		if options.ManagerName == "" {
			return nil, errors.New(errManagerName)
		}
		ctrloptions.LeaderElectionID = fmt.Sprintf("crossplane-leader-election-%s", options.ManagerName)

		if options.LeaderElectionResourceLock == "" {
			ctrloptions.LeaderElectionResourceLock = resourcelock.LeasesResourceLock
		}

		switch {
		case options.LeaderElectionNamespace != "":
			ctrloptions.LeaderElectionNamespace = options.LeaderElectionNamespace
		case options.ManagerNamespace != "":
			ctrloptions.LeaderElectionNamespace = options.ManagerNamespace
		default:
			return nil, errors.New(errManagerNamespace)
		}
	}

	return ctrl.NewManager(config, ctrloptions)
}
