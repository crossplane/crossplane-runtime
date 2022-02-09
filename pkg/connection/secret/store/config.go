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

// GetCondition of this UnstructuredConfig resource.
func (c *UnstructuredConfig) GetCondition(ct v1.ConditionType) v1.Condition {
	conditioned := v1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return v1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this UnstructuredConfig resource.
func (c *UnstructuredConfig) SetConditions(conditions ...v1.Condition) {
	conditioned := v1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetStoreConfig of this UnstructuredConfig resource.
func (c *UnstructuredConfig) GetStoreConfig() v1.SecretStoreConfig {
	cfg := v1.SecretStoreConfig{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec", &cfg); err != nil {
		return v1.SecretStoreConfig{}
	}
	return cfg
}
