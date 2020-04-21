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

package unstructured

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// CompositeOption is used to configure *Composite
type CompositeOption func(*Composite)

// WithGroupVersionKind sets the GroupVersionKind.
func WithGroupVersionKind(gvk schema.GroupVersionKind) CompositeOption {
	return func(c *Composite) {
		c.SetGroupVersionKind(gvk)
	}
}

// NewComposite returns a new *Composite configured via opts.
func NewComposite(opts ...CompositeOption) *Composite {
	c := &Composite{}
	for _, f := range opts {
		f(c)
	}
	return c
}

// An Composite is the internal representation of the resource generated
// via Crossplane definition types. It is only used for operations in the controller,
// it's not intended to be stored in the api-server.
type Composite struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Composite) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector returns the composition selector.
func (c *Composite) GetCompositionSelector() *v1.LabelSelector {
	out := &v1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector sets the composition selector.
func (c *Composite) SetCompositionSelector(sel *v1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference returns the composition reference.
func (c *Composite) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference sets the composition reference.
func (c *Composite) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetResourceReferences returns the references of composed resources.
func (c *Composite) GetResourceReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.resourceRefs", out)
	return *out
}

// SetResourceReferences sets the references of composed resources.
func (c *Composite) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	finalRefs := []corev1.ObjectReference{}
	for _, ref := range refs {
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		finalRefs = append(finalRefs, ref)
	}
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRefs", finalRefs)
}

// GetWriteConnectionSecretToReference returns the connection secret reference.
func (c *Composite) GetWriteConnectionSecretToReference() *v1alpha1.SecretReference {
	out := &v1alpha1.SecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference sets the connection secret reference.
func (c *Composite) SetWriteConnectionSecretToReference(ref *v1alpha1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this Composite.
func (c *Composite) GetCondition(ct v1alpha1.ConditionType) v1alpha1.Condition {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return v1alpha1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composite.
func (c *Composite) SetConditions(conditions ...v1alpha1.Condition) {
	conditioned := v1alpha1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// CompositeList contains a list of Composites.
type CompositeList struct {
	unstructured.UnstructuredList
}

// GetUnstructuredList returns the underlying *unstructured.UnstructuredList.
func (c *CompositeList) GetUnstructuredList() *unstructured.UnstructuredList {
	return &c.UnstructuredList
}
