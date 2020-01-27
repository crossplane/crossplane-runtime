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
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	claimFinalizerName    = "finalizer.resourceclaim.crossplane.io"
	claimReconcileTimeout = 1 * time.Minute

	aShortWait = 30 * time.Second
)

// Reasons a resource claim is or is not ready.
const (
	ReasonBinding = "Managed claim is waiting for managed resource to become bindable"
)

// Error strings.
const (
	errGetClaim          = "cannot get resource claim"
	errUpdateClaimStatus = "cannot update resource claim status"
)

// Event reasons.
const (
	reasonCannotGetResource       event.Reason = "CannotGetManagedResource"
	reasonCannotGetClass          event.Reason = "CannotGetResourceClass"
	reasonCannotConfigureResource event.Reason = "CannotConfigureManagedResource"
	reasonCannotCreateResource    event.Reason = "CannotCreateManagedResource"
	reasonCannotPropagate         event.Reason = "CannotPropagateConnectionDetails"
	reasonCannotBind              event.Reason = "CannotBindManagedResource"
	reasonCannotUnbind            event.Reason = "CannotUnbindManagedResource"

	reasonResourceNotFound event.Reason = "ManagedResourceNotFound"
	reasonCreatedResource  event.Reason = "CreatedManagedResource"
	reasonWaitingToBind    event.Reason = "WaitingToBind"
	reasonBound            event.Reason = "BoundManagedResource"
	reasonUnbound          event.Reason = "UnboundManagedResource"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of resource claim.
func ControllerName(kind string) string {
	return "claimbinding/" + strings.ToLower(kind)
}

// A ManagedConfigurator configures a resource, typically by converting it to
// a known type and populating its spec.
type ManagedConfigurator interface {
	Configure(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error
}

// A ManagedConfiguratorFn is a function that satisfies the
// ManagedConfigurator interface.
type ManagedConfiguratorFn func(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error

// Configure the supplied resource using the supplied claim and class.
func (fn ManagedConfiguratorFn) Configure(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	return fn(ctx, cm, cs, mg)
}

// A ManagedCreator creates a resource, typically by submitting it to an API
// server. ManagedCreators must not modify the supplied resource class, but are
// responsible for final modifications to the claim and resource, for example
// ensuring resource, class, claim, and owner references are set.
type ManagedCreator interface {
	Create(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error
}

// A ManagedCreatorFn is a function that satisfies the ManagedCreator interface.
type ManagedCreatorFn func(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error

// Create the supplied resource.
func (fn ManagedCreatorFn) Create(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	return fn(ctx, cm, cs, mg)
}

// A Binder binds a resource claim to a managed resource.
type Binder interface {
	// Bind the supplied Claim to the supplied Managed resource.
	Bind(ctx context.Context, cm resource.Claim, mg resource.Managed) error

	// Unbind the supplied Claim from the supplied Managed resource.
	Unbind(ctx context.Context, cm resource.Claim, mg resource.Managed) error
}

// BinderFns satisfy the Binder interface.
type BinderFns struct {
	BindFn   func(ctx context.Context, cm resource.Claim, mg resource.Managed) error
	UnbindFn func(ctx context.Context, cm resource.Claim, mg resource.Managed) error
}

// Bind the supplied Claim to the supplied Managed resource.
func (b BinderFns) Bind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	return b.BindFn(ctx, cm, mg)
}

// Unbind the supplied Claim from the supplied Managed resource.
func (b BinderFns) Unbind(ctx context.Context, cm resource.Claim, mg resource.Managed) error {
	return b.UnbindFn(ctx, cm, mg)
}

// A ClaimFinalizer finalizes the deletion of a resource claim.
type ClaimFinalizer interface {
	// AddFinalizer to the supplied Claim.
	AddFinalizer(ctx context.Context, cm resource.Claim) error

	// RemoveFinalizer from the supplied Claim.
	RemoveFinalizer(ctx context.Context, cm resource.Claim) error
}

// A ClaimFinalizerFns satisfy the ClaimFinalizer interface.
type ClaimFinalizerFns struct {
	AddFinalizerFn    func(ctx context.Context, cm resource.Claim) error
	RemoveFinalizerFn func(ctx context.Context, cm resource.Claim) error
}

// AddFinalizer to the supplied Claim.
func (f ClaimFinalizerFns) AddFinalizer(ctx context.Context, cm resource.Claim) error {
	return f.AddFinalizerFn(ctx, cm)
}

// RemoveFinalizer from the supplied Claim.
func (f ClaimFinalizerFns) RemoveFinalizer(ctx context.Context, cm resource.Claim) error {
	return f.RemoveFinalizerFn(ctx, cm)
}

// A Reconciler reconciles resource claims by creating exactly one kind of
// concrete managed resource. Each resource claim kind should create an instance
// of this controller for each managed resource kind they can bind to, using
// watch predicates to ensure each controller is responsible for exactly one
// type of resource class provisioner. Each controller must watch its subset of
// resource claims and any managed resources they control.
type Reconciler struct {
	client     client.Client
	newClaim   func() resource.Claim
	newClass   func() resource.Class
	newManaged func() resource.Managed

	// The below structs embed the set of interfaces used to implement the
	// resource claim reconciler. We do this primarily for readability, so that
	// the reconciler logic reads r.managed.Create(), r.claim.Finalize(), etc.
	managed crManaged
	claim   crClaim

	log    logging.Logger
	record event.Recorder
}

type crManaged struct {
	ManagedConfigurator
	ManagedCreator
	resource.ManagedConnectionPropagator
}

func defaultCRManaged(m manager.Manager) crManaged {
	return crManaged{
		ManagedConfigurator: ConfiguratorChain{
			ManagedConfiguratorFn(ConfigureNames),
			ManagedConfiguratorFn(ConfigureReclaimPolicy),
		},
		ManagedCreator:              NewAPIManagedCreator(m.GetClient(), m.GetScheme()),
		ManagedConnectionPropagator: resource.NewAPIManagedConnectionPropagator(m.GetClient(), m.GetScheme()),
	}
}

type crClaim struct {
	ClaimFinalizer
	Binder
}

func defaultCRClaim(m manager.Manager) crClaim {
	return crClaim{
		ClaimFinalizer: NewAPIClaimFinalizer(m.GetClient(), claimFinalizerName),
		Binder:         NewAPIStatusBinder(m.GetClient(), m.GetScheme()),
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithManagedConfigurators specifies which configurators should be used to
// configure each managed resource. Configurators will be applied in the order
// they are specified.
func WithManagedConfigurators(c ...ManagedConfigurator) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedConfigurator = ConfiguratorChain(c)
	}
}

// WithManagedCreator specifies which ManagedCreator should be used to create
// managed resources.
func WithManagedCreator(c ManagedCreator) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedCreator = c
	}
}

// WithManagedConnectionPropagator specifies which ManagedConnectionPropagator
// should be used to propagate resource connection details to their claim.
func WithManagedConnectionPropagator(p resource.ManagedConnectionPropagator) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedConnectionPropagator = p
	}
}

// WithBinder specifies which Binder should be used to bind
// resources to their claim.
func WithBinder(b Binder) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.Binder = b
	}
}

// WithClaimFinalizer specifies which ClaimFinalizer should be used to finalize
// claims when they are deleted.
func WithClaimFinalizer(f ClaimFinalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.claim.ClaimFinalizer = f
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

// NewReconciler returns a Reconciler that reconciles resource claims
// of the supplied ClaimKind with resources of the supplied ManagedKind. It
// panics if asked to reconcile a claim or resource kind that is not registered
// with the supplied manager's runtime.Scheme. The returned Reconciler will
// apply only the ObjectMetaConfigurator by default; most callers should supply
// one or more ManagedConfigurators to configure their managed resources.
func NewReconciler(m manager.Manager, of resource.ClaimKind, using resource.ClassKind, with resource.ManagedKind, o ...ReconcilerOption) *Reconciler {
	nc := func() resource.Claim {
		return resource.MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(resource.Claim)
	}
	ns := func() resource.Class {
		return resource.MustCreateObject(schema.GroupVersionKind(using), m.GetScheme()).(resource.Class)
	}
	nr := func() resource.Managed {
		return resource.MustCreateObject(schema.GroupVersionKind(with), m.GetScheme()).(resource.Managed)
	}

	// Panic early if we've been asked to reconcile a claim or resource kind
	// that has not been registered with our controller manager's scheme.
	_, _, _ = nc(), ns(), nr()

	r := &Reconciler{
		client:     m.GetClient(),
		newClaim:   nc,
		newClass:   ns,
		newManaged: nr,
		managed:    defaultCRManaged(m),
		claim:      defaultCRClaim(m),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource claim with a concrete managed resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), claimReconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get resource claim", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(claim))
	log = log.WithValues(
		"uid", claim.GetUID(),
		"version", claim.GetResourceVersion(),
		"external-name", meta.GetExternalName(claim),
	)

	managed := r.newManaged()
	if ref := claim.GetResourceReference(); ref != nil {
		record = record.WithAnnotations("managed-name", claim.GetResourceReference().Name)
		log = log.WithValues("managed-name", claim.GetResourceReference().Name)

		err := r.client.Get(ctx, meta.NamespacedNameOf(ref), managed)
		if kerrors.IsNotFound(err) {
			// If the managed resource we explicitly reference doesn't exist yet
			// we want to retry after a brief wait, in case it is created. We
			// must explicitly requeue because our EnqueueRequestForClaim
			// handler can only enqueue reconciles for managed resources that
			// have their claim reference set, so we can't expect to be queued
			// implicitly when the managed resource we want to bind to appears.
			log.Debug("Referenced managed resource not found", "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Normal(reasonResourceNotFound, "Referenced managed resource not found"))
			claim.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}
		if err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot get referenced managed resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotGetResource, err))
			claim.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}
	}

	if meta.WasDeleted(claim) {
		log = log.WithValues("deletion-timestamp", claim.GetDeletionTimestamp())

		if err := r.claim.Unbind(ctx, claim, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot unbind claim", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotUnbind, err))
			claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		log.Debug("Successfully unbound managed resource")
		record.Event(claim, event.Normal(reasonUnbound, "Successfully unbound managed resource"))

		if err := r.claim.RemoveFinalizer(ctx, claim); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot remove finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
			claim.SetConditions(v1alpha1.Deleting(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		// We've successfully deleted our claim and removed our finalizer. If we
		// assume we were the only controller that added a finalizer to this
		// claim then it should no longer exist and thus there is no point
		// trying to update its status.
		log.Debug("Successfully deleted resource claim")
		return reconcile.Result{Requeue: false}, nil
	}

	// Claim reconcilers (should) watch for either claims with a resource ref,
	// claims with a class ref, or managed resources with a claim ref. In the
	// first case the managed resource always exists by the time we get here. In
	// the second case the class reference is set. The third case exposes us to
	// a pathological scenario in which a managed resource references a claim
	// that has no resource ref or class ref, so we can't assume the class ref
	// is always set at this point.
	if !meta.WasCreated(managed) && claim.GetClassReference() != nil {
		record = record.WithAnnotations("class-name", claim.GetClassReference().Name)
		log = log.WithValues("class-name", claim.GetClassReference().Name)

		class := r.newClass()
		// Class reference should always be set by the time we get this far; we
		// set it on last reconciliation.
		if err := r.client.Get(ctx, meta.NamespacedNameOf(claim.GetClassReference()), class); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or the
			// class is (re)created.
			log.Debug("Cannot get referenced resource class", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotGetClass, err))
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.managed.Configure(ctx, claim, class, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error or some
			// issue with the resource class was resolved.
			log.Debug("Cannot configure managed resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotConfigureResource, err))
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		// We'll know our managed resource's name at this point because it was
		// set by the above configure step.
		record = record.WithAnnotations("managed-name", managed.GetName())
		log = log.WithValues("managed-name", managed.GetName())

		if err := r.managed.Create(ctx, claim, class, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot create managed resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotCreateResource, err))
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		log.Debug("Successfully created managed resource")
		record.Event(claim, event.Normal(reasonCreatedResource, "Successfully created managed resource"))
	}

	if !resource.IsBindable(managed) && !resource.IsBound(managed) {
		log.Debug("Managed resource is not yet bindable")
		record.Event(claim, event.Normal(reasonWaitingToBind, "Managed resource is not yet bindable"))

		if managed.GetClaimReference() == nil {
			// We're waiting to bind to a statically provisioned managed
			// resource. We must requeue because our EnqueueRequestForClaim
			// handler can only enqueue reconciles for managed resource updates
			// when they have their claim reference set, and that doesn't happen
			// until we bind to the managed resource we're waiting for.
			claim.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		// If this claim was not already binding we'll be requeued due to the
		// status update. Otherwise there's no need to requeue. We should be
		// watching both the resource claims and the resources we own, so we'll
		// be queued if anything changes.
		claim.SetConditions(Binding(), v1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
	}

	if resource.IsBindable(managed) {
		if err := r.managed.PropagateConnection(ctx, claim, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait in case this was a transient error, or the resource connection
			// secret is created.
			log.Debug("Cannot propagate connection details from managed resource to claim", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotPropagate, err))
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.claim.AddFinalizer(ctx, claim); err != nil {
			// If we didn't hit this error last time we'll be requeued
			// implicitly due to the status update. Otherwise we want to retry
			// after a brief wait, in case this was a transient error.
			log.Debug("Cannot add resource claim finalizer", "error", err, "requeue-after", time.Now().Add(aShortWait))
			claim.SetConditions(v1alpha1.Creating(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		if err := r.claim.Bind(ctx, claim, managed); err != nil {
			// If we didn't hit this error last time we'll be requeued implicitly
			// due to the status update. Otherwise we want to retry after a brief
			// wait, in case this was a transient error.
			log.Debug("Cannot bind to managed resource", "error", err, "requeue-after", time.Now().Add(aShortWait))
			record.Event(claim, event.Warning(reasonCannotBind, err))
			claim.SetConditions(Binding(), v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: aShortWait}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
		}

		log.Debug("Successfully bound managed resource")
		record.Event(claim, event.Normal(reasonBound, "Successfully bound managed resource"))
	}

	// No need to requeue. We should be watching both the resource claims and
	// the resources we own, so we'll be queued if anything changes.
	claim.SetConditions(v1alpha1.Available(), v1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, claim), errUpdateClaimStatus)
}

// Binding returns a condition that indicates the resource claim is currently
// waiting for its managed resource to become bindable.
func Binding() v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               v1alpha1.TypeReady,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonBinding,
	}
}
