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
	ctrl "sigs.k8s.io/controller-runtime"
)

// Options represents a crossplane specific set of Options for  controller
// managers. This internally encapsulates controller-runtime/options too.
type Options struct {
	// controller-runtime Options
	*ctrl.Options

	// ManagerNamespace determines the name of the controller manager
	// was installed (e.g. core, rbac, provider-foo, etc).
	ManagerName string

	// ManagerNamespace determines the namespace in which the crossplane
	// controller manager was installed into.
	ManagerNamespace string
}
