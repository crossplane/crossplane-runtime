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

// Package composite contains an unstructured composite resource.
package composite

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// An Option modifies an unstructured composite resource.
type Option func(*Unstructured)

// WithGroupVersionKind sets the GroupVersionKind of the unstructured composite
// resource.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(c *Unstructured) {
		c.SetGroupVersionKind(gvk)
	}
}

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured composite resource.
func WithConditions(c ...v1alpha1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// New returns a new unstructured composed resource.
func New(opts ...Option) *Unstructured {
	c := &Unstructured{unstructured.Unstructured{Object: make(map[string]interface{})}}
	for _, f := range opts {
		f(c)
	}
	return c
}

// An Unstructured composed resource.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this Composite resource.
func (c *Unstructured) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this Composite resource.
func (c *Unstructured) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this Composite resource.
func (c *Unstructured) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this Composite resource.
func (c *Unstructured) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetRequirementReference of this Composite resource.
func (c *Unstructured) GetRequirementReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.requirementRef", out); err != nil {
		return nil
	}
	return out
}

// SetRequirementReference of this Composite resource.
func (c *Unstructured) SetRequirementReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.requirementRef", ref)
}

// GetResourceReferences of this Composite resource.
func (c *Unstructured) GetResourceReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.resourceRefs", out)
	return *out
}

// SetResourceReferences of this Composite resource.
func (c *Unstructured) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	filtered := make([]corev1.ObjectReference, 0, len(refs))
	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		filtered = append(filtered, ref)
	}
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRefs", filtered)
}

// GetWriteConnectionSecretToReference of this Composite resource.
func (c *Unstructured) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	out := &v1alpha1.SecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this Composite resource.
func (c *Unstructured) SetWriteConnectionSecretToReference(ref *v1alpha1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetReclaimPolicy of this Composite resource.
func (c *Unstructured) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	s, _ := fieldpath.Pave(c.Object).GetString("spec.reclaimPolicy")
	return v1alpha1.ReclaimPolicy(s)
}

// SetReclaimPolicy of this Composite resource.
func (c *Unstructured) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.reclaimPolicy", p)
}

// GetCondition of this Composite resource.
func (c *Unstructured) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composite resource.
func (c *Unstructured) SetConditions(conditions ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}
