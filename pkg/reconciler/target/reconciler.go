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

package target

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/event"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

const (
	targetReconcileTimeout = 1 * time.Minute
	aShortWait             = 30 * time.Second
)

// Error strings
const (
	errGetTarget                 = "unable to get Target"
	errManagedResourceIsNotBound = "managed resource in Target clusterRef is unbound"
	errUpdateTarget              = "unable to update Target"
)

// Event reasons.
const (
	reasonSetSecretRef          event.Reason = "SetSecretRef"
	reasonWaitingUntilBound     event.Reason = "WaitingUntilBound"
	reasonCannotGetManaged      event.Reason = "CannotGetManaged"
	reasonCannotPropagateSecret event.Reason = "CannotPropagateSecret"
	reasonPropagatedSecret      event.Reason = "PropagatedSecret"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of target.
func ControllerName(kind string) string {
	return "target/" + strings.ToLower(kind)
}

// A Reconciler reconciles targets by propagating the secret of the
// referenced managed resource.
type Reconciler struct {
	client     client.Client
	newTarget  func() resource.Target
	newManaged func() resource.Managed

	propagator resource.ManagedConnectionPropagator

	log    logging.Logger
	record event.Recorder
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithManagedConnectionPropagator specifies which ManagedConnectionPropagator
// should be used to propagate resource connection details to their target.
func WithManagedConnectionPropagator(p resource.ManagedConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.propagator = p
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(l logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = l
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// NewReconciler returns a Reconciler that reconciles KubernetesTargets by
// propagating the referenced Kubernetes cluster's connection Secret to the
// namespace of the KubernetesTarget.
func NewReconciler(m manager.Manager, of resource.TargetKind, with resource.ManagedKind, o ...ReconcilerOption) *Reconciler {
	nt := func() resource.Target {
		return resource.MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(resource.Target)
	}
	nr := func() resource.Managed {
		return resource.MustCreateObject(schema.GroupVersionKind(with), m.GetScheme()).(resource.Managed)
	}

	// Panic early if we've been asked to reconcile a target or resource kind
	// that has not been registered with our controller manager's scheme.
	_, _ = nt(), nr()

	r := &Reconciler{
		client:     m.GetClient(),
		newTarget:  nt,
		newManaged: nr,
		propagator: resource.NewAPIManagedConnectionPropagator(m.GetClient(), m.GetScheme()),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// TODO(negz): Could we use a regular ReconcileError instead of a bespoke
// SecretPropagationError for this controller?

// Reconcile a target with a concrete managed resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), targetReconcileTimeout)
	defer cancel()

	target := r.newTarget()
	if err := r.client.Get(ctx, req.NamespacedName, target); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get target", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetTarget)
	}

	// Our watch predicates ensure we only reconcile targets with a non-nil
	// resource reference.
	record := r.record.WithAnnotations("managed-name", target.GetResourceReference().Name)
	log = log.WithValues(
		"uid", target.GetUID(),
		"version", target.GetResourceVersion(),
		"managed-name", target.GetResourceReference().Name,
	)

	if target.GetWriteConnectionSecretToReference() == nil {
		// If the ConnectionSecretRef is not set on this Target, we generate a
		// Secret name that matches the UID of the Target. We are implicitly
		// requeued because of the Target update.
		log.Debug("Set secret reference to target UID")
		record.Event(target, event.Normal(reasonSetSecretRef, "Set secret reference to target UID"))
		target.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: string(target.GetUID())})
		return reconcile.Result{}, errors.Wrap(r.client.Update(ctx, target), errUpdateTarget)
	}

	// TODO(negz): Move this above the secret ref check. Is this needed at all?
	if meta.WasDeleted(target) {
		// If the Target was deleted, there is nothing left for us to do.
		return reconcile.Result{Requeue: false}, nil
	}

	managed := r.newManaged()
	if err := r.client.Get(ctx, meta.NamespacedNameOf(target.GetResourceReference()), managed); err != nil {
		log.Debug("Cannot get referenced managed resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(target, event.Warning(reasonCannotGetManaged, err))
		target.SetConditions(v1alpha1.SecretPropagationError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if !resource.IsBound(managed) {
		// TODO(negz): Should we really consider this an error?
		log.Debug("Managed resource is not yet bound to a resource claim", "requeue-after", time.Now().Add(aShortWait))
		record.Event(target, event.Normal(reasonWaitingUntilBound, "Managed resource is not yet bound to a resource claim"))
		target.SetConditions(v1alpha1.SecretPropagationError(errors.New(errManagedResourceIsNotBound)))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if err := r.propagator.PropagateConnection(ctx, target, managed); err != nil {
		// If we fail to propagate the connection secret of a bound managed
		// resource, we try again after a short wait.
		log.Debug("Cannot propagate connection secret", "error", err, "requeue-after", time.Now().Add(aShortWait))
		record.Event(target, event.Warning(reasonCannotPropagateSecret, err))
		target.SetConditions(v1alpha1.SecretPropagationError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	// No need to requeue.
	log.Debug("Successfully propagated connection secret")
	record.Event(target, event.Normal(reasonPropagatedSecret, "Successfully propagated connection secret"))
	return reconcile.Result{Requeue: false}, nil
}
