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

// Package requirement contains an unstructured resource requirement.
package requirement

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// An Option modifies an unstructured resource requirement.
type Option func(*Unstructured)

// WithGroupVersionKind sets the GroupVersionKind of the unstructured resource
// requirement.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(c *Unstructured) {
		c.SetGroupVersionKind(gvk)
	}
}

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured resource requirement.
func WithConditions(c ...v1alpha1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// New returns a new unstructured resource requirement.
func New(opts ...Option) *Unstructured {
	c := &Unstructured{Unstructured: unstructured.Unstructured{Object: make(map[string]interface{})}}
	for _, f := range opts {
		f(c)
	}
	return c
}

// An Unstructured resource requirement.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this resource Requirement.
func (c *Unstructured) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this resource Requirement.
func (c *Unstructured) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this resource Requirement.
func (c *Unstructured) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this resource Requirement.
func (c *Unstructured) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetResourceReference of this resource Requirement.
func (c *Unstructured) GetResourceReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.resourceRef", out); err != nil {
		return nil
	}
	return out
}

// SetResourceReference of this resource Requirement.
func (c *Unstructured) SetResourceReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRef", ref)
}

// GetWriteConnectionSecretToReference of this resource Requirement.
func (c *Unstructured) GetWriteConnectionSecretToReference() *v1alpha1.LocalSecretReference {
	out := &v1alpha1.LocalSecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this resource Requirement.
func (c *Unstructured) SetWriteConnectionSecretToReference(ref *v1alpha1.LocalSecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this Requirement.
func (c *Unstructured) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Requirement.
func (c *Unstructured) SetConditions(conditions ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}
