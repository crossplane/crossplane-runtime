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
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
)

// Error strings.
const (
	errCreateManaged        = "cannot create managed resource"
	errUpdateClaim          = "cannot update resource claim"
	errGetSecret            = "cannot get managed resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errUpdateSecret         = "cannot update connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
	errUpdateManaged        = "cannot update managed resource"
	errUpdateManagedStatus  = "cannot update managed resource status"
	errDeleteManaged        = "cannot delete managed resource"
)

const claimFinalizerName = "finalizer." + claimControllerName

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
func (a *APIManagedCreator) Create(ctx context.Context, cm Claim, cs Class, mg Managed) error {
	cmr := meta.ReferenceTo(cm, MustGetKind(cm, a.typer))
	csr := meta.ReferenceTo(cs, MustGetKind(cs, a.typer))

	mg.SetClaimReference(cmr)
	mg.SetClassReference(csr)
	if err := a.client.Create(ctx, mg); err != nil {
		return errors.Wrap(err, errCreateManaged)
	}
	// Since we use GenerateName feature of ObjectMeta, final name of the
	// resource is calculated during the creation of the resource. So, we
	// can generate a complete reference only after the creation.
	mgr := meta.ReferenceTo(mg, MustGetKind(mg, a.typer))
	cm.SetResourceReference(mgr)

	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// An APIManagedConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIManagedConnectionPropagator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIManagedConnectionPropagator returns a new APIManagedConnectionPropagator.
func NewAPIManagedConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIManagedConnectionPropagator {
	return &APIManagedConnectionPropagator{client: c, typer: t}
}

// PropagateConnection details from the supplied resource to the supplied claim.
func (a *APIManagedConnectionPropagator) PropagateConnection(ctx context.Context, cm Target, mg Managed) error {
	// Either this resource does not expose a connection secret, or this claim
	// does not want one.
	if mg.GetWriteConnectionSecretToReference() == nil || cm.GetWriteConnectionSecretToReference() == nil {
		return nil
	}

	n := types.NamespacedName{
		Namespace: mg.GetWriteConnectionSecretToReference().Namespace,
		Name:      mg.GetWriteConnectionSecretToReference().Name,
	}
	mgcs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, mgcs); err != nil {
		return errors.Wrap(err, errGetSecret)
	}

	// Make sure the managed resource is the controller of the connection secret
	// it references before we propagate it. This ensures a managed resource
	// cannot use Crossplane to circumvent RBAC by propagating a secret it does
	// not own.
	if c := metav1.GetControllerOf(mgcs); c == nil || c.UID != mg.GetUID() {
		return errors.New(errSecretConflict)
	}

	cmcs := LocalConnectionSecretFor(cm, MustGetKind(cm, a.typer))
	if _, err := util.CreateOrUpdate(ctx, a.client, cmcs, func() error {
		// Inside this anonymous function cmcs could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.
		if c := metav1.GetControllerOf(cmcs); c == nil || c.UID != cm.GetUID() {
			return errors.New(errSecretConflict)
		}
		cmcs.Data = mgcs.Data
		meta.AddAnnotations(cmcs, map[string]string{
			AnnotationKeyPropagateFromNamespace: mgcs.GetNamespace(),
			AnnotationKeyPropagateFromName:      mgcs.GetName(),
			AnnotationKeyPropagateFromUID:       string(mgcs.GetUID()),
		})
		return nil
	}); err != nil {
		return errors.Wrap(err, errCreateOrUpdateSecret)
	}

	meta.AddAnnotations(mgcs, map[string]string{
		strings.Join([]string{AnnotationKeyPropagateToPrefix, string(cmcs.GetUID())}, SlashDelimeter): strings.Join([]string{cmcs.GetNamespace(), cmcs.GetName()}, SlashDelimeter),
	})

	return errors.Wrap(a.client.Update(ctx, mgcs), errUpdateSecret)
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
func (a *APIBinder) Bind(ctx context.Context, cm Claim, mg Managed) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)

	// This claim reference will already be set for dynamically provisioned
	// managed resources, but we need to make sure it's set for statically
	// provisioned resources too.
	cmr := meta.ReferenceTo(cm, MustGetKind(cm, a.typer))
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
func (a *APIBinder) Unbind(ctx context.Context, _ Claim, mg Managed) error {
	mg.SetBindingPhase(v1alpha1.BindingPhaseReleased)
	mg.SetClaimReference(nil)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errUpdateManaged)
	}

	// We go to the trouble of unbinding the managed resource before deleting it
	// because we want it to show up as "released" (not "bound") if its managed
	// resource reconciler is wedged or delayed trying to delete it.
	if mg.GetReclaimPolicy() != v1alpha1.ReclaimDelete {
		return nil
	}

	return errors.Wrap(IgnoreNotFound(a.client.Delete(ctx, mg)), errDeleteManaged)
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
func (a *APIStatusBinder) Bind(ctx context.Context, cm Claim, mg Managed) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)

	// This claim reference will already be set for dynamically provisioned
	// managed resources, but we need to make sure it's set for statically
	// provisioned resources too.
	cmr := meta.ReferenceTo(cm, MustGetKind(cm, a.typer))
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
func (a *APIStatusBinder) Unbind(ctx context.Context, _ Claim, mg Managed) error {
	mg.SetClaimReference(nil)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errUpdateManaged)
	}

	mg.SetBindingPhase(v1alpha1.BindingPhaseReleased)
	if err := a.client.Status().Update(ctx, mg); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errUpdateManagedStatus)
	}

	// We go to the trouble of unbinding the managed resource before deleting it
	// because we want it to show up as "released" (not "bound") if its managed
	// resource reconciler is wedged or delayed trying to delete it.
	if mg.GetReclaimPolicy() != v1alpha1.ReclaimDelete {
		return nil
	}

	return errors.Wrap(IgnoreNotFound(a.client.Delete(ctx, mg)), errDeleteManaged)
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
func (a *APIClaimFinalizer) AddFinalizer(ctx context.Context, cm Claim) error {
	if meta.FinalizerExists(cm, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(cm, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// RemoveFinalizer from the supplied Claim.
func (a *APIClaimFinalizer) RemoveFinalizer(ctx context.Context, cm Claim) error {
	meta.RemoveFinalizer(cm, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, cm)), errUpdateClaim)
}

// An APIManagedFinalizer adds and removes finalizers to and from a resource.
type APIManagedFinalizer struct {
	client    client.Client
	finalizer string
}

// NewAPIManagedFinalizer returns a new APIManagedFinalizer.
func NewAPIManagedFinalizer(c client.Client, finalizer string) *APIManagedFinalizer {
	return &APIManagedFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Managed resource.
func (a *APIManagedFinalizer) AddFinalizer(ctx context.Context, mg Managed) error {
	if meta.FinalizerExists(mg, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(mg, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIManagedFinalizer) RemoveFinalizer(ctx context.Context, mg Managed) error {
	meta.RemoveFinalizer(mg, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, mg)), errUpdateManaged)
}

// ManagedNameAsExternalName writes the name of the managed resource to
// the external name annotation field in order to be used as name of
// the external resource in provider.
type ManagedNameAsExternalName struct{ client client.Client }

// NewManagedNameAsExternalName returns a new ManagedNameAsExternalName.
func NewManagedNameAsExternalName(c client.Client) *ManagedNameAsExternalName {
	return &ManagedNameAsExternalName{client: c}
}

// Initialize the given managed resource.
func (a *ManagedNameAsExternalName) Initialize(ctx context.Context, mg Managed) error {
	if meta.GetExternalName(mg) != "" {
		return nil
	}
	meta.SetExternalName(mg, mg.GetName())
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}
