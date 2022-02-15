/*
 Copyright 2022 The Crossplane Authors.

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

package store

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// An ConfigOption modifies an unstructured store config resource.
type ConfigOption func(*UnstructuredConfig)

// ConfigWithGroupVersionKind sets the GroupVersionKind of the unstructured
// store config resource.
func ConfigWithGroupVersionKind(gvk schema.GroupVersionKind) ConfigOption {
	return func(c *UnstructuredConfig) {
		c.SetGroupVersionKind(gvk)
	}
}

// ConfigWithConditions returns an Option that sets the supplied conditions
// on an unstructured store config resource.
func ConfigWithConditions(c ...v1.Condition) ConfigOption {
	return func(cr *UnstructuredConfig) {
		cr.SetConditions(c...)
	}
}

// NewConfig returns a new unstructured store config resource.
func NewConfig(opts ...ConfigOption) *UnstructuredConfig {
	c := &UnstructuredConfig{unstructured.Unstructured{Object: make(map[string]interface{})}}
	for _, f := range opts {
		f(c)
	}
	return c
}

// An UnstructuredConfig is an unstructured store config resource.
type UnstructuredConfig struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (cr *UnstructuredConfig) GetUnstructured() *unstructured.Unstructured {
	return &cr.Unstructured
}

// GetCondition of this UnstructuredConfig resource.
func (cr *UnstructuredConfig) GetCondition(ct v1.ConditionType) v1.Condition {
	conditioned := v1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned); err != nil {
		return v1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this UnstructuredConfig resource.
func (cr *UnstructuredConfig) SetConditions(conditions ...v1.Condition) {
	conditioned := v1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(cr.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetStoreConfig of this UnstructuredConfig resource.
func (cr *UnstructuredConfig) GetStoreConfig() v1.SecretStoreConfig {
	cfg := v1.SecretStoreConfig{}
	if err := fieldpath.Pave(cr.Object).GetValueInto("spec", &cfg); err != nil {
		return v1.SecretStoreConfig{}
	}
	return cfg
}
