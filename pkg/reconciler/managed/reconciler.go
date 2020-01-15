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
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

const (
	managedControllerName = "managedresource.crossplane.io"
	managedFinalizerName  = "finalizer." + managedControllerName

	managedReconcileTimeout = 1 * time.Minute

	defaultManagedShortWait = 30 * time.Second
	defaultManagedLongWait  = 1 * time.Minute
)

// Error strings.
const (
	errGetManaged       = "cannot get managed resource"
	errReconcileConnect = "connect failed"
	errReconcileObserve = "observe failed"
	errReconcileCreate  = "create failed"
	errReconcileUpdate  = "update failed"
	errReconcileDelete  = "delete failed"
)

var log = logging.Logger.WithName("controller")

// ConnectionDetails created or updated during an operation on an external
// resource, for example usernames, passwords, endpoints, ports, etc.
type ConnectionDetails map[string][]byte

// A ManagedConnectionPublisher manages the supplied ConnectionDetails for the
// supplied Managed resource. ManagedPublishers must handle the case in which
// the supplied ConnectionDetails are empty.
type ManagedConnectionPublisher interface {
	// PublishConnection details for the supplied Managed resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publicing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error

	// UnpublishConnection details for the supplied Managed resource.
	UnpublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
}

// ManagedConnectionPublisherFns is the pluggable struct to produce objects with ManagedConnectionPublisher interface.
type ManagedConnectionPublisherFns struct {
	PublishConnectionFn   func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
	UnpublishConnectionFn func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
}

// PublishConnection details for the supplied Managed resource.
func (fn ManagedConnectionPublisherFns) PublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
	return fn.PublishConnectionFn(ctx, mg, c)
}

// UnpublishConnection details for the supplied Managed resource.
func (fn ManagedConnectionPublisherFns) UnpublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
	return fn.UnpublishConnectionFn(ctx, mg, c)
}

// A ManagedInitializer establishes ownership of the supplied Managed resource.
// This typically involves the operations that are run before calling any
// ExternalClient methods.
type ManagedInitializer interface {
	Initialize(ctx context.Context, mg resource.Managed) error
}

// A InitializerChain chains multiple managed initializers.
type InitializerChain []ManagedInitializer

// Initialize calls each ManagedInitializer serially. It returns the first
// error it encounters, if any.
func (cc InitializerChain) Initialize(ctx context.Context, mg resource.Managed) error {
	for _, c := range cc {
		if err := c.Initialize(ctx, mg); err != nil {
			return err
		}
	}
	return nil
}

// A ManagedFinalizer finalizes the deletion of a resource claim.
type ManagedFinalizer interface {
	// AddFinalizer to the supplied Managed resource.
	AddFinalizer(ctx context.Context, mg resource.Managed) error

	// RemoveFinalizer from the supplied Managed resource.
	RemoveFinalizer(ctx context.Context, mg resource.Managed) error
}

// A ManagedFinalizerFns satisfy the ManagedFinalizer interface.
type ManagedFinalizerFns struct {
	AddFinalizerFn    func(ctx context.Context, mg resource.Managed) error
	RemoveFinalizerFn func(ctx context.Context, mg resource.Managed) error
}

// AddFinalizer to the supplied Managed resource.
func (f ManagedFinalizerFns) AddFinalizer(ctx context.Context, mg resource.Managed) error {
	return f.AddFinalizerFn(ctx, mg)
}

// RemoveFinalizer from the supplied Managed resource.
func (f ManagedFinalizerFns) RemoveFinalizer(ctx context.Context, mg resource.Managed) error {
	return f.RemoveFinalizerFn(ctx, mg)
}

// A ManagedInitializerFn is a function that satisfies the ManagedInitializer
// interface.
type ManagedInitializerFn func(ctx context.Context, mg resource.Managed) error

// Initialize calls ManagedInitializerFn function.
func (m ManagedInitializerFn) Initialize(ctx context.Context, mg resource.Managed) error {
	return m(ctx, mg)
}

// A ManagedReferenceResolver resolves references to other managed resources.
type ManagedReferenceResolver interface {
	// ResolveReferences finds all fields in the supplied CanReference that are
	// references to Kubernetes resources, then uses the fields of those
	// resources to update corresponding fields in CanReference, for example
	// setting .spec.network to the name of the Network resource specified as
	// .spec.networkRef.
	ResolveReferences(ctx context.Context, res CanReference) error
}

// A ManagedReferenceResolverFn is a function that satisfies the
// ManagedReferenceResolver interface.
type ManagedReferenceResolverFn func(context.Context, CanReference) error

// ResolveReferences calls ManagedReferenceResolverFn function
func (m ManagedReferenceResolverFn) ResolveReferences(ctx context.Context, res CanReference) error {
	return m(ctx, res)
}

// An ExternalConnecter produces a new ExternalClient given the supplied
// Managed resource.
type ExternalConnecter interface {
	// Connect to the provider specified by the supplied managed resource and
	// produce an ExternalClient.
	Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error)
}

// An ExternalConnectorFn is a function that satisfies the ExternalConnecter
// interface.
type ExternalConnectorFn func(ctx context.Context, mg resource.Managed) (ExternalClient, error)

// Connect to the provider specified by the supplied managed resource and
// produce an ExternalClient.
func (ec ExternalConnectorFn) Connect(ctx context.Context, mg resource.Managed) (ExternalClient, error) {
	return ec(ctx, mg)
}

// An ExternalClient manages the lifecycle of an external resource.
// None of the calls here should be blocking. All of the calls should be
// idempotent. For example, Create call should not return AlreadyExists error
// if it's called again with the same parameters or Delete call should not
// return error if there is an ongoing deletion or resource does not exist.
type ExternalClient interface {
	// Observe the external resource the supplied Managed resource represents,
	// if any. Observe implementations must not modify the external resource,
	// but may update the supplied Managed resource to reflect the state of the
	// external resource.
	Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error)

	// Create an external resource per the specifications of the supplied
	// Managed resource. Called when Observe reports that the associated
	// external resource does not exist.
	Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error)

	// Update the external resource represented by the supplied Managed
	// resource, if necessary. Called unless Observe reports that the
	// associated external resource is up to date.
	Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)

	// Delete the external resource upon deletion of its associated Managed
	// resource. Called when the managed resource has been deleted.
	Delete(ctx context.Context, mg resource.Managed) error
}

// ExternalClientFns are a series of functions that satisfy the ExternalClient
// interface.
type ExternalClientFns struct {
	ObserveFn func(ctx context.Context, mg resource.Managed) (ExternalObservation, error)
	CreateFn  func(ctx context.Context, mg resource.Managed) (ExternalCreation, error)
	UpdateFn  func(ctx context.Context, mg resource.Managed) (ExternalUpdate, error)
	DeleteFn  func(ctx context.Context, mg resource.Managed) error
}

// Observe the external resource the supplied Managed resource represents, if
// any.
func (e ExternalClientFns) Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error) {
	return e.ObserveFn(ctx, mg)
}

// Create an external resource per the specifications of the supplied Managed
// resource.
func (e ExternalClientFns) Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error) {
	return e.CreateFn(ctx, mg)
}

// Update the external resource represented by the supplied Managed resource, if
// necessary.
func (e ExternalClientFns) Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error) {
	return e.UpdateFn(ctx, mg)
}

// Delete the external resource upon deletion of its associated Managed
// resource.
func (e ExternalClientFns) Delete(ctx context.Context, mg resource.Managed) error {
	return e.DeleteFn(ctx, mg)
}

// A NopConnecter does nothing.
type NopConnecter struct{}

// Connect returns a NopClient. It never returns an error.
func (c *NopConnecter) Connect(_ context.Context, _ resource.Managed) (ExternalClient, error) {
	return &NopClient{}, nil
}

// A NopClient does nothing.
type NopClient struct{}

// Observe does nothing. It returns an empty ExternalObservation and no error.
func (c *NopClient) Observe(ctx context.Context, mg resource.Managed) (ExternalObservation, error) {
	return ExternalObservation{}, nil
}

// Create does nothing. It returns an empty ExternalCreation and no error.
func (c *NopClient) Create(ctx context.Context, mg resource.Managed) (ExternalCreation, error) {
	return ExternalCreation{}, nil
}

// Update does nothing. It returns an empty ExternalUpdate and no error.
func (c *NopClient) Update(ctx context.Context, mg resource.Managed) (ExternalUpdate, error) {
	return ExternalUpdate{}, nil
}

// Delete does nothing. It never returns an error.
func (c *NopClient) Delete(ctx context.Context, mg resource.Managed) error { return nil }

// An ExternalObservation is the result of an observation of an external
// resource.
type ExternalObservation struct {
	ResourceExists    bool
	ResourceUpToDate  bool
	ConnectionDetails ConnectionDetails
}

// An ExternalCreation is the result of the creation of an external resource.
type ExternalCreation struct {
	ConnectionDetails ConnectionDetails
}

// An ExternalUpdate is the result of an update to an external resource.
type ExternalUpdate struct {
	ConnectionDetails ConnectionDetails
}

// A Reconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type Reconciler struct {
	client     client.Client
	newManaged func() resource.Managed

	shortWait time.Duration
	longWait  time.Duration

	// The below structs embed the set of interfaces used to implement the
	// managed resource reconciler. We do this primarily for readability, so
	// that the reconciler logic reads r.external.Connect(),
	// r.managed.Delete(), etc.
	external mrExternal
	managed  mrManaged
}

type mrManaged struct {
	ManagedConnectionPublisher
	ManagedFinalizer
	ManagedInitializer
	ManagedReferenceResolver
}

func defaultMRManaged(m manager.Manager) mrManaged {
	return mrManaged{
		ManagedConnectionPublisher: NewAPISecretPublisher(m.GetClient(), m.GetScheme()),
		ManagedFinalizer:           NewAPIManagedFinalizer(m.GetClient(), managedFinalizerName),
		ManagedInitializer:         NewManagedNameAsExternalName(m.GetClient()),
		ManagedReferenceResolver:   NewAPIManagedReferenceResolver(m.GetClient()),
	}
}

type mrExternal struct {
	ExternalConnecter
}

func defaultMRExternal() mrExternal {
	return mrExternal{
		ExternalConnecter: &NopConnecter{},
	}
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithShortWait specifies how long the Reconciler should wait before queueing a
// new reconciliation in 'short wait' scenarios. The Reconciler requeues after a
// short wait when it knows it is waiting for an external operation to complete,
// or when it encounters a potentially temporary error.
func WithShortWait(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.shortWait = after
	}
}

// WithLongWait specifies how long the Reconciler should wait before queueing a
// new reconciliation in 'long wait' scenarios. The Reconciler requeues after a
// long wait when it is not actively waiting for an external operation, but
// wishes to check whether an existing external resource needs to be synced to
// its Crossplane Managed resource.
func WithLongWait(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.longWait = after
	}
}

// WithExternalConnecter specifies how the Reconciler should connect to the API
// used to sync and delete external resources.
func WithExternalConnecter(c ExternalConnecter) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnecter = c
	}
}

// WithManagedConnectionPublishers specifies how the Reconciler should publish
// its connection details such as credentials and endpoints.
func WithManagedConnectionPublishers(p ...ManagedConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedConnectionPublisher = PublisherChain(p)
	}
}

// WithManagedInitializers specifies how the Reconciler should initialize a
// managed resource before calling any of the ExternalClient functions.
func WithManagedInitializers(i ...ManagedInitializer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedInitializer = InitializerChain(i)
	}
}

// WithManagedFinalizer specifies how the Reconciler should add and remove
// finalizers to and from the managed resource.
func WithManagedFinalizer(f ManagedFinalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedFinalizer = f
	}
}

// WithManagedReferenceResolver specifies how the Reconciler should resolve any
// inter-resource references it encounters while reconciling managed resources.
func WithManagedReferenceResolver(rr ManagedReferenceResolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ManagedReferenceResolver = rr
	}
}

// NewReconciler returns a Reconciler that reconciles managed resources of the
// supplied ManagedKind with resources in an external system such as a cloud
// provider API. It panics if asked to reconcile a managed resource kind that is
// not registered with the supplied manager's runtime.Scheme. The returned
// Reconciler reconciles with a dummy, no-op 'external system' by default;
// callers should supply an ExternalConnector that returns an ExternalClient
// capable of managing resources in a real system.
func NewReconciler(m manager.Manager, of resource.ManagedKind, o ...ReconcilerOption) *Reconciler {
	nm := func() resource.Managed {
		return resource.MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(resource.Managed)
	}

	// Panic early if we've been asked to reconcile a resource kind that has not
	// been registered with our controller manager's scheme.
	_ = nm()

	r := &Reconciler{
		client:     m.GetClient(),
		newManaged: nm,
		shortWait:  defaultManagedShortWait,
		longWait:   defaultManagedLongWait,
		managed:    defaultMRManaged(m),
		external:   defaultMRExternal(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a managed resource with an external resource.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is a well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log.V(logging.Debug).Info("Reconciling", "controller", managedControllerName, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), managedReconcileTimeout)
	defer cancel()

	managed := r.newManaged()
	if err := r.client.Get(ctx, req.NamespacedName, managed); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetManaged)
	}

	external, err := r.external.Connect(ctx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider or its secret are missing
		// or invalid. If this is first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errReconcileConnect)))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.Initialize(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition.
		// If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We resolve any references before observing our external resource because
	// in some rare examples we need a spec field to make the observe call, and
	// that spec field could be set by a reference.
	//
	// We do not resolve references when being deleted because it is likely that
	// the resources we reference are also being deleted, and would thus block
	// resolution due to being unready or non-existent. It is unlikely (but not
	// impossible) that we need to resolve a reference in order to process a
	// delete, and that reference is stale at delete time.
	if !meta.WasDeleted(managed) {
		if err := r.managed.ResolveReferences(ctx, managed); err != nil {
			condition := v1alpha1.ReconcileError(err)
			if IsReferencesAccessError(err) {
				condition = v1alpha1.ReferenceResolutionBlocked(err)
			}

			// If any of our referenced resources are not yet ready (or if we
			// encountered an error resolving them) we want to try again after a
			// short wait. If this is the first time we encounter this situation
			// we'll be requeued implicitly due to the status update.
			managed.SetConditions(condition)
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		managed.SetConditions(v1alpha1.ReferenceResolutionSuccess())
	}

	observation, err := external.Observe(ctx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider credentials are invalid
		// or insufficient for observing the external resource type we're
		// concerned with. If this is the first time we encounter this issue
		// we'll be requeued implicitly when we update our status with the new
		// error condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errReconcileObserve)))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if meta.WasDeleted(managed) {
		if observation.ResourceExists && managed.GetReclaimPolicy() == v1alpha1.ReclaimDelete {
			if err := external.Delete(ctx, managed); err != nil {
				// We'll hit this condition if we can't delete our external
				// resource, for example if our provider credentials don't have
				// access to delete it. If this is the first time we encounter this
				// issue we'll be requeued implicitly when we update our status with
				// the new error condition. If not, we want to try again after a
				// short wait.
				managed.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errReconcileDelete)))
				return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
			}

			// We've successfully requested deletion of our external resource.
			// We queue another reconcile after a short wait rather than
			// immediately finalizing our delete in order to verify that the
			// external resource was actually deleted. If it no longer exists
			// we'll skip this block on the next reconcile and proceed to
			// unpublish and finalize. If it still exists we'll re-enter this
			// block and try again.
			managed.SetConditions(v1alpha1.ReconcileSuccess())
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.UnpublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we want to try again after a short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(resource.IgnoreNotFound(r.client.Status().Update(ctx, managed)), errUpdateManagedStatus)
		}
		if err := r.managed.RemoveFinalizer(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we want to try again after a short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(resource.IgnoreNotFound(r.client.Status().Update(ctx, managed)), errUpdateManagedStatus)
		}

		// We've successfully deleted our external resource (if necessary) and
		// removed our finalizer. If we assume we were the only controller that
		// added a finalizer to this resource then it should no longer exist and
		// thus there is no point trying to update its status.
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.managed.PublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.AddFinalizer(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(resource.IgnoreNotFound(r.client.Status().Update(ctx, managed)), errUpdateManagedStatus)
	}

	if !observation.ResourceExists {
		creation, err := external.Create(ctx, managed)
		if err != nil {
			// We'll hit this condition if we can't create our external
			// resource, for example if our provider credentials don't have
			// access to create it. If this is the first time we encounter this
			// issue we'll be requeued implicitly when we update our status with
			// the new error condition. If not, we want to try again after a
			// short wait.
			managed.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errReconcileCreate)))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		if err := r.managed.PublishConnection(ctx, managed, creation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we want to try again after a short wait.
			managed.SetConditions(v1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully created our external resource. In many cases the
		// creation process takes a little time to finish. We requeue a short
		// wait in order to observe the external resource to determine whether
		// it's ready for use.
		managed.SetConditions(v1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if observation.ResourceUpToDate {
		// We did not need to create, update, or delete our external resource.
		// Per the below issue nothing will notify us if and when the external
		// resource we manage changes, so we requeue a speculative reconcile
		// after a long wait in order to observe it and react accordingly.
		// https://github.com/crossplaneio/crossplane/issues/289
		managed.SetConditions(v1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.longWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	update, err := external.Update(ctx, managed)
	if err != nil {
		// We'll hit this condition if we can't update our external resource,
		// for example if our provider credentials don't have access to update
		// it. If this is the first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errReconcileUpdate)))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.PublishConnection(ctx, managed, update.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we want to try again after a short wait.
		managed.SetConditions(v1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: r.shortWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We've successfully updated our external resource. Per the below issue
	// nothing will notify us if and when the external resource we manage
	// changes, so we requeue a speculative reconcile after a long wait in order
	// to observe it and react accordingly.
	// https://github.com/crossplaneio/crossplane/issues/289
	managed.SetConditions(v1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: r.longWait}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
}
