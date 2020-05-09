/*
Copyright 2019 The Crossplane Authors.

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

package claimbinding

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateManaged       = "cannot create managed resource"
	errUpdateClaim         = "cannot update resource claim"
	errUpdateManaged       = "cannot update managed resource"
	errUpdateManagedStatus = "cannot update managed resource status"
	errDeleteManaged       = "cannot delete managed resource"
	errBindMismatch        = "refusing to bind to managed resource that does not reference resource claim"
	errUnbindMismatch      = "refusing to 'unbind' from managed resource that does not reference resource claim"
	errBindControlled      = "refusing to bind to managed resource that is controlled by another resource"
)

// An APIManagedCreator creates resources by submitting them to a Kubernetes
// API server.
type APIManagedCreator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIManagedCreator returns a new APIManagedCreator.
func NewAPIManagedCreator(c client.Client, t runtime.ObjectTyper) *APIManagedCreator {
	return &APIManagedCreator{client: c, typer: t}
}

// Create the supplied resource using the supplied class and claim.
func (a *APIManagedCreator) Create(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	cmr := meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer))
	csr := meta.ReferenceTo(cs, resource.MustGetKind(cs, a.typer))

	mg.SetClaimReference(cmr)
	mg.SetClassReference(csr)
	if err := a.client.Create(ctx, mg); err != nil {
		return errors.Wrap(err, errCreateManaged)
	}
	// Since we use GenerateName feature of ObjectMeta, final name of the
	// resource is calculated during the creation of the resource. So, we
	// can generate a complete reference only after the creation.
	mgr := meta.ReferenceTo(mg, resource.MustGetKind(mg, a.typer))
	cm.SetResourceReference(mgr)

	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// An APIBinder binds resources to claims by updating them in a Kubernetes API
// server. Note that APIBinder does not support objects using the status
// subresource; such objects should use APIStatusBinder.
type APIBinder struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIBinder returns a new APIBinder.
func NewAPIBinder(c client.Client, t runtime.ObjectTyper) *APIBinder {
	return &APIBinder{client: c, typer: t}
}

// Bind the supplied resource to the supplied claim.
func (a *APIBinder) Bind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	// A managed resource that was statically provisioned by an infrastructure
	// operator should not have a controller reference. We assume a managed
	// resource with a controller reference is part of a composite resource or a
	// stack, and therefore not available to be claimed.
	if metav1.GetControllerOf(mg) != nil {
		return errors.New(errBindControlled)
	}

	// Note that we allow a claim to bind to a managed resource with a nil claim
	// reference in order to enable the static provisioning case in which a
	// managed resource is provisioned ahead of time and is not associated with
	// any claim.
	if r := mg.GetClaimReference(); r != nil && !equal(meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer)), r) {
		return errors.New(errBindMismatch)
	}

	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)

	// This claim reference will already be set for dynamically provisioned
	// managed resources, but we need to make sure it's set for statically
	// provisioned resources too.
	cmr := meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer))
	mg.SetClaimReference(cmr)
	mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManaged)
	}

	if meta.GetExternalName(mg) == "" {
		return nil
	}

	// Propagate back the final name of the external resource to the claim.
	meta.SetExternalName(cm, meta.GetExternalName(mg))
	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// Unbind the supplied Claim from the supplied Managed resource by removing the
// managed resource's claim reference, transitioning it to binding phase
// "Released", and if the managed resource's reclaim policy is "Delete",
// deleting it.
func (a *APIBinder) Unbind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	if !equal(meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer)), mg.GetClaimReference()) {
		return errors.New(errUnbindMismatch)
	}

	mg.SetBindingPhase(v1alpha1.BindingPhaseReleased)
	mg.SetClaimReference(nil)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(resource.IgnoreNotFound(err), errUpdateManaged)
	}

	// We go to the trouble of unbinding the managed resource before deleting it
	// because we want it to show up as "released" (not "bound") if its managed
	// resource reconciler is wedged or delayed trying to delete it.
	if mg.GetReclaimPolicy() != v1alpha1.ReclaimDelete {
		return nil
	}

	return errors.Wrap(resource.IgnoreNotFound(a.client.Delete(ctx, mg)), errDeleteManaged)
}

// An APIStatusBinder binds resources to claims by updating them in a
// Kubernetes API server. Note that APIStatusBinder does not support
// objects that do not use the status subresource; such objects should use
// APIBinder.
type APIStatusBinder struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIStatusBinder returns a new APIStatusBinder.
func NewAPIStatusBinder(c client.Client, t runtime.ObjectTyper) *APIStatusBinder {
	return &APIStatusBinder{client: c, typer: t}
}

// Bind the supplied resource to the supplied claim.
func (a *APIStatusBinder) Bind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	// A managed resource that was statically provisioned by an infrastructure
	// operator should not have a controller reference. We assume a managed
	// resource with a controller reference is part of a composite resource or a
	// stack, and therefore not available to be claimed.
	if metav1.GetControllerOf(mg) != nil {
		return errors.New(errBindControlled)
	}

	// Note that we allow a claim to bind to a managed resource with a nil claim
	// reference in order to enable the static provisioning case in which a
	// managed resource is provisioned ahead of time and is not associated with
	// any claim.
	if r := mg.GetClaimReference(); r != nil && !equal(meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer)), r) {
		return errors.New(errBindMismatch)
	}

	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)

	// This claim reference will already be set for dynamically provisioned
	// managed resources, but we need to make sure it's set for statically
	// provisioned resources too.
	cmr := meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer))
	mg.SetClaimReference(cmr)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManaged)
	}

	mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Status().Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManagedStatus)
	}

	if meta.GetExternalName(mg) == "" {
		return nil
	}

	// Propagate back the final name of the external resource to the claim.
	meta.SetExternalName(cm, meta.GetExternalName(mg))
	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// Unbind the supplied Claim from the supplied Managed resource by removing the
// managed resource's claim reference, transitioning it to binding phase
// "Released", and if the managed resource's reclaim policy is "Delete",
// deleting it.
func (a *APIStatusBinder) Unbind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	if !equal(meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer)), mg.GetClaimReference()) {
		return errors.New(errUnbindMismatch)
	}

	mg.SetClaimReference(nil)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(resource.IgnoreNotFound(err), errUpdateManaged)
	}

	mg.SetBindingPhase(v1alpha1.BindingPhaseReleased)
	if err := a.client.Status().Update(ctx, mg); err != nil {
		return errors.Wrap(resource.IgnoreNotFound(err), errUpdateManagedStatus)
	}

	// We go to the trouble of unbinding the managed resource before deleting it
	// because we want it to show up as "released" (not "bound") if its managed
	// resource reconciler is wedged or delayed trying to delete it.
	if mg.GetReclaimPolicy() != v1alpha1.ReclaimDelete {
		return nil
	}

	return errors.Wrap(resource.IgnoreNotFound(a.client.Delete(ctx, mg)), errDeleteManaged)
}

// equal returns true if the supplied object references are considered equal. We
// consider two otherwise matching references with different UIDs to be equal,
// presuming that they are both references to a particular object that has been
// deleted and recreated, e.g. due to being restored from a backup.
//
// TODO(negz): If we switch to a reference that only has the fields we care
// about (GVK, namespace, and name) we can just use struct equality.
// https://github.com/crossplane/crossplane-runtime/issues/49
func equal(a, b *corev1.ObjectReference) bool {
	switch {
	case a == nil || b == nil:
		return a == b
	case a.APIVersion != b.APIVersion:
		return false
	case a.Kind != b.Kind:
		return false
	case a.Namespace != b.Namespace:
		return false
	}
	return a.Name == b.Name
}
