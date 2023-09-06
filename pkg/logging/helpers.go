/*
Copyright 2023 The Crossplane Authors.

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

package logging

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ForResource returns logging values for a resource.
func ForResource(object client.Object) []string {
	ret := make([]string, 0, 10)
	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Kind == "" {
		gvk.Kind = fmt.Sprintf("%T", object) // best effort for native Go types
	}
	ret = append(ret,
		"name", object.GetName(),
		"kind", gvk.Kind,
		"version", gvk.Version,
		"group", gvk.Group,
	)
	if ns := object.GetNamespace(); ns != "" {
		ret = append(ret, "namespace", ns)
	}

	return ret
}
