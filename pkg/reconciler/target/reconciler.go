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
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

const (
	targetControllerName   = "kubernetestarget.crossplane.io"
	targetReconcileTimeout = 1 * time.Minute
	aShortWait             = 30 * time.Second

	errGetTarget                 = "unable to get Target"
	errManagedResourceIsNotBound = "managed resource in Target clusterRef is unbound"
	errUpdateTarget              = "unable to update Target"
)

var log = logging.Logger.WithName("controller")

// A Reconciler reconciles targets by propagating the secret of the
// referenced managed resource.
type Reconciler struct {
	client     client.Client
	newTarget  func() resource.Target
	newManaged func() resource.Managed

	propagator resource.ManagedConnectionPropagator
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
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a target with a concrete managed resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("Reconciling", "controller", targetControllerName, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), targetReconcileTimeout)
	defer cancel()

	target := r.newTarget()
	if err := r.client.Get(ctx, req.NamespacedName, target); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetTarget)
	}

	if target.GetWriteConnectionSecretToReference() == nil {
		// If the ConnectionSecretRef is not set on this Target, we generate a
		// Secret name that matches the UID of the Target. We are implicitly
		// requeued because of the Target update.
		target.SetWriteConnectionSecretToReference(&v1alpha1.LocalSecretReference{Name: string(target.GetUID())})
		return reconcile.Result{}, errors.Wrap(r.client.Update(ctx, target), errUpdateTarget)
	}

	if meta.WasDeleted(target) {
		// If the Target was deleted, there is nothing left for us to do.
		return reconcile.Result{Requeue: false}, nil
	}

	managed := r.newManaged()
	if err := r.client.Get(ctx, meta.NamespacedNameOf(target.GetResourceReference()), managed); err != nil {
		target.SetConditions(v1alpha1.SecretPropagationError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if !resource.IsBound(managed) {
		target.SetConditions(v1alpha1.SecretPropagationError(errors.New(errManagedResourceIsNotBound)))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if err := r.propagator.PropagateConnection(ctx, target, managed); err != nil {
		// If we fail to propagate the connection secret of a bound managed resource, we try again after a short wait.
		target.SetConditions(v1alpha1.SecretPropagationError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	// No need to requeue.
	return reconcile.Result{Requeue: false}, nil
}
