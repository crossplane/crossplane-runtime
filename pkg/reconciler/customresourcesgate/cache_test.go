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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransformStripCRDSchema(t *testing.T) {
	type args struct {
		obj any
	}

	type want struct {
		obj any
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"StripsSchemaManagedFieldsAndAnnotation": {
			reason: "Should strip OpenAPI schemas, ManagedFields, and last-applied-configuration annotation",
			args: args{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/last-applied-configuration": `{"very":"large","json":"blob"}`,
							"other-annotation": "keep-me",
						},
						ManagedFields: []metav1.ManagedFieldsEntry{
							{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
						},
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:   "v1",
								Served: true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"spec": {Type: "object"},
										},
									},
								},
							},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
			want: want{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "testresources.example.com",
						Annotations: map[string]string{
							"other-annotation": "keep-me",
						},
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:   "v1",
								Served: true,
								Schema: nil,
							},
						},
					},
					Status: apiextensionsv1.CustomResourceDefinitionStatus{
						Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
							{
								Type:   apiextensionsv1.Established,
								Status: apiextensionsv1.ConditionTrue,
							},
						},
					},
				},
			},
		},
		"MultipleVersions": {
			reason: "Should strip schemas from all versions",
			args: args{
				obj: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{
								Name:   "v1",
								Served: true,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
								},
							},
							{
								Name:   "v1beta1",
								Served: false,
								Schema: &apiextensionsv1.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
								},
							},
						},
					},
				},
			},
			want: want{
				obj: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
						Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
							{Name: "v1", Served: true, Schema: nil},
							{Name: "v1beta1", Served: false, Schema: nil},
						},
					},
				},
			},
		},
		"NoVersions": {
			reason: "Should handle CRD with no versions without panicking",
			args: args{
				obj: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
			want: want{
				obj: &apiextensionsv1.CustomResourceDefinition{
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
		},
		"NilAnnotations": {
			reason: "Should handle CRD with nil annotations map without panicking",
			args: args{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
			want: want{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test.example.com",
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
		},
		"NoLastAppliedAnnotation": {
			reason: "Should leave other annotations intact when last-applied-configuration is absent",
			args: args{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"custom-annotation": "value",
						},
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
			want: want{
				obj: &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"custom-annotation": "value",
						},
					},
					Spec: apiextensionsv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: "TestResource"},
					},
				},
			},
		},
		"NonCRDObject": {
			reason: "Should return non-CRD objects unchanged",
			args: args{
				obj: "not-a-crd",
			},
			want: want{
				obj: "not-a-crd",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := TransformStripCRDSchema(tc.args.obj)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nTransformStripCRDSchema(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.obj, got); diff != "" {
				t.Errorf("%s\nTransformStripCRDSchema(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
