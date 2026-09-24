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
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
)

// Schema specifies the schema version of a composite resource's Crossplane
// machinery fields.
type Schema int

const (
	// SchemaModern indicates a modern Namespaced or Cluster scope composite
	// resource. Modern composite resources nest all Crossplane machinery fields
	// under spec.crossplane and status.crossplane, and can't be claimed.
	SchemaModern Schema = iota

	// SchemaLegacy indicates a LegacyCluster scope composite resource. Legacy
	// composite resources don't nest Crossplane machinery fields - they're set
	// directly under spec and status. Legacy composite resources can be claimed.
	SchemaLegacy
)

// An Option modifies an unstructured composite resource.
type Option func(*Unstructured)

// WithGroupVersionKind sets the GroupVersionKind of the composite resource.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(c *Unstructured) {
		c.SetGroupVersionKind(gvk)
	}
}

// WithConditions sets the supplied conditions on the composite resource.
func WithConditions(c ...xpv2.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// WithSchema sets the schema of the composite resource.
func WithSchema(s Schema) Option {
	return func(c *Unstructured) {
		c.Schema = s
	}
}

// New returns a new unstructured composite resource.
func New(opts ...Option) *Unstructured {
	c := &Unstructured{Unstructured: unstructured.Unstructured{Object: make(map[string]any)}}
	for _, f := range opts {
		f(c)
	}

	return c
}

// +k8s:deepcopy-gen=true
// +kubebuilder:object:root=true

// An Unstructured composite resource.
type Unstructured struct {
	unstructured.Unstructured

	Schema Schema
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this composite resource.
func (c *Unstructured) GetCompositionSelector() *metav1.LabelSelector {
	path := "spec.crossplane.compositionSelector"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionSelector"
	}

	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto(path, out); err != nil {
		return nil
	}

	return out
}

// SetCompositionSelector of this composite resource.
func (c *Unstructured) SetCompositionSelector(sel *metav1.LabelSelector) {
	path := "spec.crossplane.compositionSelector"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionSelector"
	}

	_ = fieldpath.Pave(c.Object).SetValue(path, sel)
}

// GetCompositionReference of this composite resource.
func (c *Unstructured) GetCompositionReference() *corev1.ObjectReference {
	path := "spec.crossplane.compositionRef"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRef"
	}

	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto(path, out); err != nil {
		return nil
	}

	return out
}

// SetCompositionReference of this composite resource.
func (c *Unstructured) SetCompositionReference(ref *corev1.ObjectReference) {
	path := "spec.crossplane.compositionRef"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRef"
	}

	_ = fieldpath.Pave(c.Object).SetValue(path, ref)
}

// GetCompositionRevisionReference of this composite resource.
func (c *Unstructured) GetCompositionRevisionReference() *corev1.LocalObjectReference {
	path := "spec.crossplane.compositionRevisionRef"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRevisionRef"
	}

	out := &corev1.LocalObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto(path, out); err != nil {
		return nil
	}

	return out
}

// SetCompositionRevisionReference of this composite resource.
func (c *Unstructured) SetCompositionRevisionReference(ref *corev1.LocalObjectReference) {
	path := "spec.crossplane.compositionRevisionRef"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRevisionRef"
	}

	_ = fieldpath.Pave(c.Object).SetValue(path, ref)
}

// GetCompositionRevisionSelector of this resource claim.
func (c *Unstructured) GetCompositionRevisionSelector() *metav1.LabelSelector {
	path := "spec.crossplane.compositionRevisionSelector"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRevisionSelector"
	}

	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto(path, out); err != nil {
		return nil
	}

	return out
}

// SetCompositionRevisionSelector of this resource claim.
func (c *Unstructured) SetCompositionRevisionSelector(sel *metav1.LabelSelector) {
	path := "spec.crossplane.compositionRevisionSelector"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionRevisionSelector"
	}

	_ = fieldpath.Pave(c.Object).SetValue(path, sel)
}

// SetCompositionUpdatePolicy of this composite resource.
func (c *Unstructured) SetCompositionUpdatePolicy(p *xpv2.UpdatePolicy) {
	path := "spec.crossplane.compositionUpdatePolicy"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionUpdatePolicy"
	}

	_ = fieldpath.Pave(c.Object).SetValue(path, p)
}

// GetCompositionUpdatePolicy of this composite resource.
func (c *Unstructured) GetCompositionUpdatePolicy() *xpv2.UpdatePolicy {
	path := "spec.crossplane.compositionUpdatePolicy"
	if c.Schema == SchemaLegacy {
		path = "spec.compositionUpdatePolicy"
	}

	p, err := fieldpath.Pave(c.Object).GetString(path)
	if err != nil {
		return nil
	}

	out := xpv2.UpdatePolicy(p)

	return &out
}

// GetClaimReference of this composite resource.
func (c *Unstructured) GetClaimReference() *reference.Claim {
	// Only legacy XRs support claims.
	if c.Schema != SchemaLegacy {
		return nil
	}

	out := &reference.Claim{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.claimRef", out); err != nil {
		return nil
	}

	return out
}

// SetClaimReference of this composite resource.
func (c *Unstructured) SetClaimReference(ref *reference.Claim) {
	// Only legacy XRs support claims.
	if c.Schema != SchemaLegacy {
		return
	}

	_ = fieldpath.Pave(c.Object).SetValue("spec.claimRef", ref)
}

// resourceRefsPath is where this composite resource keeps its composed
// resource references.
func (c *Unstructured) resourceRefsPath() string {
	if c.Schema == SchemaLegacy {
		return "spec.resourceRefs"
	}

	return "spec.crossplane.resourceRefs"
}

// GetComposedResourceReferences of this composite resource. Unlike
// GetResourceReferences these carry the composition resource name and the
// ordering constraints declared over each resource.
func (c *Unstructured) GetComposedResourceReferences() []reference.Composed {
	out := &[]reference.Composed{}
	_ = fieldpath.Pave(c.Object).GetValueInto(c.resourceRefsPath(), out)

	return *out
}

// SetComposedResourceReferences of this composite resource.
func (c *Unstructured) SetComposedResourceReferences(refs []reference.Composed) {
	filtered := make([]reference.Composed, 0, len(refs))

	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.APIVersion == "" && ref.Kind == "" && ref.Name == "" && ref.Namespace == "" &&
			ref.ResourceName == "" && len(ref.DependsOn) == 0 {
			continue
		}

		filtered = append(filtered, ref)
	}

	_ = fieldpath.Pave(c.Object).SetValue(c.resourceRefsPath(), filtered)
}

// pendingResourcesPath is where this composite resource records what the
// ordering graph is holding back. The same path for every schema: status
// isn't shared with fields the user writes, so there's nothing for a
// crossplane stanza to keep it apart from.
const pendingResourcesPath = "status.pendingResources"

// GetPendingResources of this composite resource: the composed resources
// ordering will not create or delete yet, and why.
func (c *Unstructured) GetPendingResources() []reference.Pending {
	out := &[]reference.Pending{}
	_ = fieldpath.Pave(c.Object).GetValueInto(pendingResourcesPath, out)

	return *out
}

// SetPendingResources of this composite resource. Setting an empty slice
// removes the field, so a composite that is no longer waiting for anything
// doesn't carry an empty array saying so.
func (c *Unstructured) SetPendingResources(pending []reference.Pending) {
	if len(pending) == 0 {
		_ = fieldpath.Pave(c.Object).DeleteField(pendingResourcesPath)
		return
	}

	_ = fieldpath.Pave(c.Object).SetValue(pendingResourcesPath, pending)
}

// GetResourceReferences of this composite resource.
func (c *Unstructured) GetResourceReferences() []corev1.ObjectReference {
	refs := c.GetComposedResourceReferences()

	out := make([]corev1.ObjectReference, len(refs))
	for i, ref := range refs {
		out[i] = corev1.ObjectReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			Namespace:  ref.Namespace,
		}
	}

	return out
}

// SetResourceReferences of this composite resource. Ordering fields already
// recorded for a referenced resource are preserved, so a caller that doesn't
// know about them can't erase them.
func (c *Unstructured) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}

	// A recorded reference is matched to the supplied one by object identity.
	// A reference that has no identity - one that carries only a composition
	// resource name, or only the resources it depends on - cannot be matched,
	// because there is no ObjectReference for it to arrive on. Carrying those
	// over wholesale is what stops this method erasing ordering that its
	// caller has no way to supply.
	existing := map[corev1.ObjectReference]reference.Composed{}
	unidentified := []reference.Composed{}

	for _, ref := range c.GetComposedResourceReferences() {
		k := corev1.ObjectReference{APIVersion: ref.APIVersion, Kind: ref.Kind, Name: ref.Name, Namespace: ref.Namespace}
		if k == empty {
			unidentified = append(unidentified, ref)
			continue
		}

		existing[k] = ref
	}

	filtered := make([]reference.Composed, 0, len(refs)+len(unidentified))

	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}

		filtered = append(filtered, reference.Composed{
			APIVersion:   ref.APIVersion,
			Kind:         ref.Kind,
			Name:         ref.Name,
			Namespace:    ref.Namespace,
			ResourceName: existing[ref].ResourceName,
			DependsOn:    existing[ref].DependsOn,
		})
	}

	c.SetComposedResourceReferences(append(filtered, unidentified...))
}

// GetReference returns reference to this composite.
func (c *Unstructured) GetReference() *reference.Composite {
	ref := &reference.Composite{
		APIVersion: c.GetAPIVersion(),
		Kind:       c.GetKind(),
		Name:       c.GetName(),
	}

	if c.GetNamespace() != "" {
		ref.Namespace = ptr.To(c.GetNamespace())
	}

	return ref
}

// TODO(negz): Ideally we'd use LocalSecretReference for namespaced XRs. As is
// we'll return a SecretReference with an empty namespace if the XR doesn't
// actually have a spec.crossplane.writeConnectionSecretToRef.namespace field.

// GetWriteConnectionSecretToReference of this composite resource.
func (c *Unstructured) GetWriteConnectionSecretToReference() *xpv2.SecretReference {
	// Only legacy XRs support connection secrets.
	if c.Schema != SchemaLegacy {
		return nil
	}

	out := &xpv2.SecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}

	return out
}

// SetWriteConnectionSecretToReference of this composite resource.
func (c *Unstructured) SetWriteConnectionSecretToReference(ref *xpv2.SecretReference) {
	// Only legacy XRs support connection secrets.
	if c.Schema != SchemaLegacy {
		return
	}

	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this composite resource.
func (c *Unstructured) GetCondition(ct xpv2.ConditionType) xpv2.Condition {
	conditioned := xpv2.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv2.Condition{}
	}

	return conditioned.GetCondition(ct)
}

// SetConditions of this composite resource.
func (c *Unstructured) SetConditions(conditions ...xpv2.Condition) {
	conditioned := xpv2.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetConditions of this composite resource.
func (c *Unstructured) GetConditions() []xpv2.Condition {
	conditioned := xpv2.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)

	return conditioned.Conditions
}

// GetConnectionDetailsLastPublishedTime of this composite resource.
func (c *Unstructured) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	// Only legacy XRs support connection details.
	if c.Schema != SchemaLegacy {
		return nil
	}

	out := &metav1.Time{}
	if err := fieldpath.Pave(c.Object).GetValueInto("status.connectionDetails.lastPublishedTime", out); err != nil {
		return nil
	}

	return out
}

// SetConnectionDetailsLastPublishedTime of this composite resource.
func (c *Unstructured) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	// Only legacy XRs support connection details.
	if c.Schema != SchemaLegacy {
		return
	}

	_ = fieldpath.Pave(c.Object).SetValue("status.connectionDetails.lastPublishedTime", t)
}

// SetObservedGeneration of this composite resource claim.
func (c *Unstructured) SetObservedGeneration(generation int64) {
	status := &xpv2.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	status.SetObservedGeneration(generation)
	_ = fieldpath.Pave(c.Object).SetValue("status.observedGeneration", status.ObservedGeneration)
}

// GetObservedGeneration of this composite resource claim.
func (c *Unstructured) GetObservedGeneration() int64 {
	status := &xpv2.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)

	return status.GetObservedGeneration()
}

// SetLastHandledReconcileAt of this composite resource.
func (c *Unstructured) SetLastHandledReconcileAt(token string) {
	_ = fieldpath.Pave(c.Object).SetValue("status.lastHandledReconcileAt", token)
}

// GetLastHandledReconcileAt of this composite resource.
func (c *Unstructured) GetLastHandledReconcileAt() string {
	v, err := fieldpath.Pave(c.Object).GetString("status.lastHandledReconcileAt")
	if err != nil {
		return ""
	}
	return v
}

// SetClaimConditionTypes of this composite resource. You cannot set system
// condition types such as Ready, Synced or Healthy as claim conditions.
func (c *Unstructured) SetClaimConditionTypes(in ...xpv2.ConditionType) error {
	// Only legacy XRs support claims.
	if c.Schema != SchemaLegacy {
		return nil
	}

	ts := c.GetClaimConditionTypes()

	m := make(map[xpv2.ConditionType]bool, len(ts))
	for _, t := range ts {
		m[t] = true
	}

	for _, t := range in {
		if xpv2.IsSystemConditionType(t) {
			return errors.Errorf("cannot set system condition %s as a claim condition", t)
		}

		if m[t] {
			continue
		}

		m[t] = true
		ts = append(ts, t)
	}

	_ = fieldpath.Pave(c.Object).SetValue("status.claimConditionTypes", ts)

	return nil
}

// GetClaimConditionTypes of this composite resource.
func (c *Unstructured) GetClaimConditionTypes() []xpv2.ConditionType {
	// Only legacy XRs support claims.
	if c.Schema != SchemaLegacy {
		return nil
	}

	cs := []xpv2.ConditionType{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status.claimConditionTypes", &cs)

	return cs
}
