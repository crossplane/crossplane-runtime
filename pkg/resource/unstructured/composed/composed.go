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

// Package composed contains an unstructured composed resource.
package composed

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

// An Option modifies an unstructured composed resource.
type Option func(resource *Unstructured)

// FromReference returns an Option that propagates the metadata in the supplied
// reference to an unstructured composed resource.
func FromReference(ref corev1.ObjectReference) Option {
	return func(cr *Unstructured) {
		cr.SetGroupVersionKind(ref.GroupVersionKind())
		cr.SetName(ref.Name)
		cr.SetNamespace(ref.Namespace)
		cr.SetUID(ref.UID)
	}
}

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured composed resource.
func WithConditions(c ...xpv1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// New returns a new unstructured composed resource.
func New(opts ...Option) *Unstructured {
	cr := &Unstructured{unstructured.Unstructured{Object: make(map[string]any)}}
	for _, f := range opts {
		f(cr)
	}
	return cr
}

// An Unstructured composed resource.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (cr *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &cr.Unstructured
}

// GetCondition of this Composed resource.
func (cr *Unstructured) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composed resource.
func (cr *Unstructured) SetConditions(c ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(c...)
	_ = fieldpath.Pave(cr.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetWriteConnectionSecretToReference of this Composed resource.
func (cr *Unstructured) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	if err := fieldpath.Pave(cr.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this Composed resource.
func (cr *Unstructured) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	_ = fieldpath.Pave(cr.Object).SetValue("spec.writeConnectionSecretToRef", r)
}

// GetPublishConnectionDetailsTo of this Composed resource.
func (cr *Unstructured) GetPublishConnectionDetailsTo() *xpv1.PublishConnectionDetailsTo {
	out := &xpv1.PublishConnectionDetailsTo{}
	if err := fieldpath.Pave(cr.Object).GetValueInto("spec.publishConnectionDetailsTo", out); err != nil {
		return nil
	}
	return out
}

// SetPublishConnectionDetailsTo of this Composed resource.
func (cr *Unstructured) SetPublishConnectionDetailsTo(ref *xpv1.PublishConnectionDetailsTo) {
	_ = fieldpath.Pave(cr.Object).SetValue("spec.publishConnectionDetailsTo", ref)
}
