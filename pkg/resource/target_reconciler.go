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
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
)

const (
	targetControllerName   = "kubernetestarget.crossplane.io"
	targetReconcileTimeout = 1 * time.Minute

	errGetTarget                 = "unable to get Target"
	errMissingSecretRef          = "no secretRef specified for Target with a clusterRef"
	errManagedResourceIsNotBound = "managed resource in Target clusterRef is unbound"
	errUpdateTarget              = "unable to update Target"
)

// A TargetKind contains the type metadata for a kind of target resource.
type TargetKind schema.GroupVersionKind

// A TargetReconciler reconciles targets by propagating the secret of the
// referenced managed resource.
type TargetReconciler struct {
	client     client.Client
	newTarget  func() Target
	newManaged func() Managed

	propagator ManagedConnectionPropagator
}

// NewTargetReconciler returns a Reconciler that reconciles
// KubernetesTarget by setting their ConnectionSecretRef to that of the
// referenced Kubernetes cluster implementation.
func NewTargetReconciler(m manager.Manager, of TargetKind, with ManagedKind) *TargetReconciler {
	nt := func() Target { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Target) }
	nr := func() Managed { return MustCreateObject(schema.GroupVersionKind(with), m.GetScheme()).(Managed) }

	// Panic early if we've been asked to reconcile a target or resource kind
	// that has not been registered with our controller manager's scheme.
	_, _ = nt(), nr()

	r := &TargetReconciler{
		client:     m.GetClient(),
		newTarget:  nt,
		newManaged: nr,
		propagator: NewAPIManagedConnectionPropagator(m.GetClient(), m.GetScheme()),
	}

	return r
}

// Reconcile a target with a concrete managed resource.
func (r *TargetReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("Reconciling", "controller", targetControllerName, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), targetReconcileTimeout)
	defer cancel()

	target := r.newTarget()
	if err := r.client.Get(ctx, req.NamespacedName, target); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetTarget)
	}

	if target.GetWriteConnectionSecretToReference() == nil {
		// If the ConnectionSecretRef is not set on this Target, we will not
		// know where to propagate the secret. We do not explicitly requeue
		// because we have a watch on updates to the KubernetesTarget.
		target.SetConditions(v1alpha1.SecretPropagatedError(errors.New(errMissingSecretRef)))
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if meta.WasDeleted(target) {
		// If the Target was deleted, there is nothing left for us to do.
		return reconcile.Result{Requeue: false}, nil
	}

	managed := r.newManaged()
	if err := r.client.Get(ctx, meta.NamespacedNameOf(target.GetResourceReference()), managed); err != nil {
		target.SetConditions(v1alpha1.SecretPropagatedError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if !IsBound(managed) {
		target.SetConditions(v1alpha1.SecretPropagatedError(errors.New(errManagedResourceIsNotBound)))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	if err := r.propagator.PropagateConnection(ctx, target, managed); err != nil {
		// If we fail to propagate the connection secret of a bound managed resource, we try again after a short wait.
		target.SetConditions(v1alpha1.SecretPropagatedError(err))
		return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, target), errUpdateTarget)
	}

	// No need to requeue.
	return reconcile.Result{Requeue: false}, nil
}
