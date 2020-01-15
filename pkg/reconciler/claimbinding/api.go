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

package resource

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateManaged       = "cannot create managed resource"
	errUpdateClaim         = "cannot update resource claim"
	errUpdateManaged       = "cannot update managed resource"
	errUpdateManagedStatus = "cannot update managed resource status"
	errDeleteManaged       = "cannot delete managed resource"
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
func (a *APIBinder) Unbind(ctx context.Context, _ resource.Claim, mg resource.Managed) error {
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
func (a *APIStatusBinder) Unbind(ctx context.Context, _ resource.Claim, mg resource.Managed) error {
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

// An APIClaimFinalizer adds and removes finalizers to and from a claim.
type APIClaimFinalizer struct {
	client    client.Client
	finalizer string
}

// NewAPIClaimFinalizer returns a new APIClaimFinalizer.
func NewAPIClaimFinalizer(c client.Client, finalizer string) *APIClaimFinalizer {
	return &APIClaimFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Claim.
func (a *APIClaimFinalizer) AddFinalizer(ctx context.Context, cm resource.Claim) error {
	if meta.FinalizerExists(cm, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(cm, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// RemoveFinalizer from the supplied Claim.
func (a *APIClaimFinalizer) RemoveFinalizer(ctx context.Context, cm resource.Claim) error {
	meta.RemoveFinalizer(cm, a.finalizer)
	return errors.Wrap(resource.IgnoreNotFound(a.client.Update(ctx, cm)), errUpdateClaim)
}
