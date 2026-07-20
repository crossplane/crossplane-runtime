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

package customresourcesgate

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TransformStripCRDSchema is a cache.TransformFunc that removes heavy fields
// from CustomResourceDefinition objects before they are stored in the informer
// cache. It strips:
//   - Spec.Versions[].Schema (OpenAPI v3 validation schemas)
//   - ObjectMeta.ManagedFields
//   - The "kubectl.kubernetes.io/last-applied-configuration" annotation
//
// This significantly reduces memory usage in clusters with many CRDs. The CRD
// gate reconciler only needs basic metadata (group, kind, version names, served
// status) and status conditions to function correctly.
//
// Usage:
//
//	cache.Options{
//	    ByObject: map[client.Object]cache.ByObject{
//	        &apiextensionsv1.CustomResourceDefinition{}: {
//	            Transform: customresourcesgate.TransformStripCRDSchema,
//	        },
//	    },
//	}
func TransformStripCRDSchema(obj any) (any, error) {
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return obj, nil
	}

	for i := range crd.Spec.Versions {
		crd.Spec.Versions[i].Schema = nil
	}

	crd.ManagedFields = nil

	delete(crd.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

	return crd, nil
}
