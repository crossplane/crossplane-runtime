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

// Package providerconfig provides a reconciler that manages the lifecycle of a
// ProviderConfig.
package providerconfig

import (
	"context"
	"strings"
	"time"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

const (
	finalizer = "in-use.crossplane.io"
	shortWait = 30 * time.Second
	timeout   = 2 * time.Minute

	errGetPC        = "cannot get ProviderConfig"
	errListPCUs     = "cannot list ProviderConfigUsages"
	errDeletePCU    = "cannot delete ProviderConfigUsage"
	errGetPCUOwner  = "cannot get ProviderConfigUsage owner"
	errReleasePCU   = "cannot release ProviderConfigUsage"
	errUpdate       = "cannot update ProviderConfig"
	errUpdateStatus = "cannot update ProviderConfig status"
)

// Event reasons.
const (
	reasonAccount event.Reason = "UsageAccounting"
)

// Condition types and reasons.
const (
	TypeTerminating xpv2.ConditionType   = "Terminating"
	ReasonInUse     xpv2.ConditionReason = "InUse"
)

// Terminating indicates a ProviderConfig has been deleted, but that the
// deletion is being blocked because it is still in use.
func Terminating() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeTerminating,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInUse,
	}
}

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of managed resource.
func ControllerName(kind string) string {
	return "providerconfig/" + strings.ToLower(kind)
}

// A Reconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type Reconciler struct {
	client client.Client
	// apiReader bypasses the cache when checking a usage's owner.
	apiReader client.Reader

	newConfig    func() resource.ProviderConfig
	newUsageList func() resource.ProviderConfigUsageList

	legacyPCU bool

	log    logging.Logger
	record event.Recorder
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

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

// NewReconciler returns a Reconciler of ProviderConfigs.
func NewReconciler(m manager.Manager, of resource.ProviderConfigKinds, o ...ReconcilerOption) *Reconciler {
	nc := func() resource.ProviderConfig {
		//nolint:forcetypeassert // If this isn't a ProviderConfig it's a programming error and we want to panic.
		return resource.MustCreateObject(of.Config, m.GetScheme()).(resource.ProviderConfig)
	}
	nul := func() resource.ProviderConfigUsageList {
		//nolint:forcetypeassert // If this isn't a ProviderConfigUsage it's a programming error and we want to panic.
		return resource.MustCreateObject(of.UsageList, m.GetScheme()).(resource.ProviderConfigUsageList)
	}
	_, isLegacyPCU := resource.MustCreateObject(of.Usage, m.GetScheme()).(resource.LegacyProviderConfigUsage)

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_, _ = nc(), nul()

	r := &Reconciler{
		client:    m.GetClient(),
		apiReader: m.GetAPIReader(),

		newConfig:    nc,
		newUsageList: nul,
		legacyPCU:    isLegacyPCU,

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a ProviderConfig by accounting for the managed resources that are
// using it, and ensuring it cannot be deleted until it is no longer in use.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pc := r.newConfig()
	if err := r.client.Get(ctx, req.NamespacedName, pc); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetPC, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPC)
	}

	log = log.WithValues(
		"uid", pc.GetUID(),
		"version", pc.GetResourceVersion(),
		"name", pc.GetName(),
		"namespace", pc.GetNamespace(),
	)

	l := r.newUsageList()

	matchingLabels := client.MatchingLabels{
		xpv2.LabelKeyProviderName: pc.GetName(),
	}

	if !r.legacyPCU {
		matchingLabels[xpv2.LabelKeyProviderKind] = pc.GetObjectKind().GroupVersionKind().Kind
	}

	listOpts := []client.ListOption{matchingLabels}
	if pc.GetNamespace() != "" {
		listOpts = append(listOpts, client.InNamespace(pc.GetNamespace()))
	}

	if err := r.client.List(ctx, l, listOpts...); err != nil {
		log.Debug(errListPCUs, "error", err)
		r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errListPCUs)))

		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	users := int64(len(l.GetItems()))

	// Requeue while waiting for an owner because this controller does not watch
	// managed resources.
	recheck := false

	for _, pcu := range l.GetItems() {
		if metav1.GetControllerOf(pcu) == nil {
			if err := r.deleteOrphanedUsage(ctx, log, pc, pcu); err != nil {
				return reconcile.Result{RequeueAfter: shortWait}, nil
			}

			users--

			continue
		}

		released, wait := r.releaseUsage(ctx, log, pc, pcu)
		if wait {
			recheck = true
		}

		if released {
			users--
		}
	}

	log = log.WithValues("usages", users)

	res := reconcile.Result{Requeue: false}
	if recheck {
		res = reconcile.Result{RequeueAfter: shortWait}
	}

	if meta.WasDeleted(pc) {
		if users > 0 {
			msg := "Blocking deletion while usages still exist"

			log.Debug(msg)
			r.record.Event(pc, event.Warning(reasonAccount, errors.New(msg)))

			// Requeue if a terminating usage is waiting for its owner to finish.
			pc.SetUsers(users)
			pc.SetConditions(Terminating().WithMessage(msg))

			return res, errors.Wrap(r.client.Status().Update(ctx, pc), errUpdateStatus)
		}

		meta.RemoveFinalizer(pc, finalizer)

		if err := r.client.Update(ctx, pc); err != nil {
			r.log.Debug(errUpdate, "error", err)
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		// We've been deleted - there's no more work to do.
		return reconcile.Result{Requeue: false}, nil
	}

	meta.AddFinalizer(pc, finalizer)

	if err := r.client.Update(ctx, pc); err != nil {
		r.log.Debug(errUpdate, "error", err)
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// Requeue only while waiting for a managed resource to finish deleting.
	pc.SetUsers(users)

	return res, errors.Wrap(r.client.Status().Update(ctx, pc), errUpdateStatus)
}

// deleteOrphanedUsage removes a ProviderConfigUsage that has no controller
// reference. Usages should always have one; if this one has none it's probably
// been stripped off (e.g. by a Velero restore). It's either stale, or will be
// recreated next time the relevant managed resource connects.
func (r *Reconciler) deleteOrphanedUsage(ctx context.Context, log logging.Logger, pc resource.ProviderConfig, pcu resource.ProviderConfigUsage) error {
	// Release our finalizer first. Without an owner reference we can't tell
	// which managed resource would remove it, so deleting the usage while
	// it's still finalized would leave it terminating forever - and in turn
	// block its namespace from being deleted.
	if meta.FinalizerExists(pcu, resource.ProviderConfigUsageFinalizer) {
		meta.RemoveFinalizer(pcu, resource.ProviderConfigUsageFinalizer)

		if err := r.client.Update(ctx, pcu); resource.IgnoreNotFound(err) != nil {
			log.Debug(errReleasePCU, "error", err)
			r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errReleasePCU)))

			return errors.Wrap(err, errReleasePCU)
		}
	}

	if err := r.client.Delete(ctx, pcu); resource.IgnoreNotFound(err) != nil {
		log.Debug(errDeletePCU, "error", err)
		r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errDeletePCU)))

		return errors.Wrap(err, errDeletePCU)
	}

	return nil
}

// releaseUsage releases a terminating ProviderConfigUsage whose owner is gone,
// was replaced, or has finished tearing down its external resource - nothing
// else will. It reports whether the usage no longer counts as a user of the
// ProviderConfig, and whether the ProviderConfig should be rechecked later
// because the usage is still waiting for its owner.
func (r *Reconciler) releaseUsage(ctx context.Context, log logging.Logger, pc resource.ProviderConfig, pcu resource.ProviderConfigUsage) (bool, bool) {
	if !meta.WasDeleted(pcu) || !meta.FinalizerExists(pcu, resource.ProviderConfigUsageFinalizer) {
		return false, false
	}

	ref := metav1.GetControllerOf(pcu)

	// Read the owner from the API server, not the cache. This reconciler
	// runs inside providers that configure their own manager cache; an
	// informer restricted by selector or namespace, or one that isn't
	// synced, reports a live owner as not found. Releasing the usage on a
	// false not-found lets the ProviderConfig and its credentials go while
	// the owner is still deleting its external resource.
	owner := &unstructured.Unstructured{}
	owner.SetAPIVersion(ref.APIVersion)
	owner.SetKind(ref.Kind)

	err := r.apiReader.Get(ctx, client.ObjectKey{Namespace: pcu.GetNamespace(), Name: ref.Name}, owner)
	if err != nil && !apierrors.IsNotFound(err) {
		// We can't tell whether the owner still needs its ProviderConfig -
		// its kind may no longer be served, for example. Keep blocking
		// deletion and surface why, rather than risking orphaned external
		// resources.
		log.Debug(errGetPCUOwner, "error", err)
		r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errGetPCUOwner)))

		return false, true
	}

	if err == nil && owner.GetUID() == ref.UID && !ownerFinishedTeardown(owner) {
		// Recheck the usage after its owner has had time to finish deleting.
		return false, true
	}

	// The owner is gone, was recreated, or has finished tearing down its
	// external resource. Nothing will release this usage but us.
	meta.RemoveFinalizer(pcu, resource.ProviderConfigUsageFinalizer)
	if err := r.client.Update(ctx, pcu); resource.IgnoreNotFound(err) != nil {
		log.Debug(errReleasePCU, "error", err)
		r.record.Event(pc, event.Warning(reasonAccount, errors.Wrap(err, errReleasePCU)))

		return false, true
	}

	return true, false
}

// ownerFinishedTeardown returns true if the supplied owner is being deleted and
// has no finalizers other than Kubernetes deletion propagation finalizers.
func ownerFinishedTeardown(owner metav1.Object) bool {
	return meta.WasDeleted(owner) && len(meta.NonGCFinalizers(owner)) == 0
}
