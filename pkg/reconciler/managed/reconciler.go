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

package managed

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	managedFinalizerName = "finalizer.managedresource.crossplane.io"
	reconcileGracePeriod = 30 * time.Second
	reconcileTimeout     = 1 * time.Minute

	defaultpollInterval = 1 * time.Minute
)

// Error strings.
const (
	errGetManaged               = "cannot get managed resource"
	errUpdateManagedAfterCreate = "cannot update managed resource. this may have resulted in a leaked external resource"
	errReconcileConnect         = "connect failed"
	errReconcileObserve         = "observe failed"
	errReconcileCreate          = "create failed"
	errReconcileUpdate          = "update failed"
	errReconcileDelete          = "delete failed"
)

// Event reasons.
const (
	reasonCannotConnect       event.Reason = "CannotConnectToProvider"
	reasonCannotInitialize    event.Reason = "CannotInitializeManagedResource"
	reasonCannotResolveRefs   event.Reason = "CannotResolveResourceReferences"
	reasonCannotObserve       event.Reason = "CannotObserveExternalResource"
	reasonCannotCreate        event.Reason = "CannotCreateExternalResource"
	reasonCannotDelete        event.Reason = "CannotDeleteExternalResource"
	reasonCannotPublish       event.Reason = "CannotPublishConnectionDetails"
	reasonCannotUnpublish     event.Reason = "CannotUnpublishConnectionDetails"
	reasonCannotUpdate        event.Reason = "CannotUpdateExternalResource"
	reasonCannotUpdateManaged event.Reason = "CannotUpdateManagedResource"

	reasonDeleted event.Reason = "DeletedExternalResource"
	reasonCreated event.Reason = "CreatedExternalResource"
	reasonUpdated event.Reason = "UpdatedExternalResource"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of managed resource.
func ControllerName(kind string) string {
	return "managed/" + strings.ToLower(kind)
}

// ConnectionDetails created or updated during an operation on an external
// resource, for example usernames, passwords, endpoints, ports, etc.
type ConnectionDetails map[string][]byte

// A ConnectionPublisher manages the supplied ConnectionDetails for the
// supplied Managed resource. ManagedPublishers must handle the case in which
// the supplied ConnectionDetails are empty.
type ConnectionPublisher interface {
	// PublishConnection details for the supplied Managed resource. Publishing
	// must be additive; i.e. if details (a, b, c) are published, subsequently
	// publicing details (b, c, d) should update (b, c) but not remove a.
	PublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error

	// UnpublishConnection details for the supplied Managed resource.
	UnpublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
}

// ConnectionPublisherFns is the pluggable struct to produce objects with ConnectionPublisher interface.
type ConnectionPublisherFns struct {
	PublishConnectionFn   func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
	UnpublishConnectionFn func(ctx context.Context, mg resource.Managed, c ConnectionDetails) error
}

// PublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) PublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
	return fn.PublishConnectionFn(ctx, mg, c)
}

// UnpublishConnection details for the supplied Managed resource.
func (fn ConnectionPublisherFns) UnpublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
	return fn.UnpublishConnectionFn(ctx, mg, c)
}

// A Initializer establishes ownership of the supplied Managed resource.
// This typically involves the operations that are run before calling any
// ExternalClient methods.
type Initializer interface {
	Initialize(ctx context.Context, mg resource.Managed) error
}

// A InitializerChain chains multiple managed initializers.
type InitializerChain []Initializer

// Initialize calls each Initializer serially. It returns the first
// error it encounters, if any.
func (cc InitializerChain) Initialize(ctx context.Context, mg resource.Managed) error {
	for _, c := range cc {
		if err := c.Initialize(ctx, mg); err != nil {
			return err
		}
	}
	return nil
}

// A InitializerFn is a function that satisfies the Initializer
// interface.
type InitializerFn func(ctx context.Context, mg resource.Managed) error

// Initialize calls InitializerFn function.
func (m InitializerFn) Initialize(ctx context.Context, mg resource.Managed) error {
	return m(ctx, mg)
}

// A ReferenceResolver resolves references to other managed resources.
type ReferenceResolver interface {
	// ResolveReferences resolves all fields in the supplied managed resource
	// that are references to other managed resources by updating corresponding
	// fields, for example setting spec.network to the Network resource
	// specified by spec.networkRef.name.
	ResolveReferences(ctx context.Context, mg resource.Managed) error
}

// A ReferenceResolverFn is a function that satisfies the
// ReferenceResolver interface.
type ReferenceResolverFn func(context.Context, resource.Managed) error

// ResolveReferences calls ReferenceResolverFn function
func (m ReferenceResolverFn) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	return m(ctx, mg)
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
	// ResourceExists must be true if a corresponding external resource exists
	// for the managed resource. Typically this is proven by the presence of an
	// external resource of the expected kind whose unique identifier matches
	// the managed resource's external name. Crossplane uses this information to
	// determine whether it needs to create or delete the external resource.
	ResourceExists bool

	// ResourceUpToDate should be true if the corresponding external resource
	// appears to be up-to-date - i.e. updating the external resource to match
	// the desired state of the managed resource would be a no-op. Keep in mind
	// that often only a subset of external resource fields can be updated.
	// Crossplane uses this information to determine whether it needs to update
	// the external resource.
	ResourceUpToDate bool

	// ResourceLateInitialized should be true if the managed resource's spec was
	// updated during its observation. A Crossplane provider may update a
	// managed resource's spec fields after it is created or updated, as long as
	// the updates are limited to setting previously unset fields, and adding
	// keys to maps. Crossplane uses this information to determine whether
	// changes to the spec were made during observation that must be persisted.
	// Note that changes to the spec will be persisted before changes to the
	// status, and that pending changes to the status may be lost when the spec
	// is persisted. Status changes will be persisted by the first subsequent
	// observation that _does not_ late initialize the managed resource, so it
	// is important that Observe implementations do not late initialize the
	// resource every time they are called.
	ResourceLateInitialized bool

	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails
}

// An ExternalCreation is the result of the creation of an external resource.
type ExternalCreation struct {
	// ExternalNameAssigned is true if the Create operation resulted in a change
	// in the external name annotation. If that's the case, we need to issue a
	// spec update and make sure it goes through so that we don't lose the identifier
	// of the resource we just created.
	ExternalNameAssigned bool

	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails
}

// An ExternalUpdate is the result of an update to an external resource.
type ExternalUpdate struct {
	// ConnectionDetails required to connect to this resource. These details
	// are a set that is collated throughout the managed resource's lifecycle -
	// i.e. returning new connection details will have no affect on old details
	// unless an existing key is overwritten. Crossplane may publish these
	// credentials to a store (e.g. a Secret).
	ConnectionDetails ConnectionDetails
}

// A Reconciler reconciles managed resources by creating and managing the
// lifecycle of an external resource, i.e. a resource in an external system such
// as a cloud provider API. Each controller must watch the managed resource kind
// for which it is responsible.
type Reconciler struct {
	client     client.Client
	newManaged func() resource.Managed

	pollInterval time.Duration
	timeout      time.Duration

	// The below structs embed the set of interfaces used to implement the
	// managed resource reconciler. We do this primarily for readability, so
	// that the reconciler logic reads r.external.Connect(),
	// r.managed.Delete(), etc.
	external mrExternal
	managed  mrManaged

	log    logging.Logger
	record event.Recorder
}

type mrManaged struct {
	ConnectionPublisher
	resource.Finalizer
	Initializer
	ReferenceResolver
}

func defaultMRManaged(m manager.Manager) mrManaged {
	return mrManaged{
		ConnectionPublisher: NewAPISecretPublisher(m.GetClient(), m.GetScheme()),
		Finalizer:           resource.NewAPIFinalizer(m.GetClient(), managedFinalizerName),
		Initializer:         NewNameAsExternalName(m.GetClient()),
		ReferenceResolver:   NewAPISimpleReferenceResolver(m.GetClient()),
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

// WithTimeout specifies the timeout duration cumulatively for all the calls happen
// in the reconciliation function. In case the deadline exceeds, reconciler will
// still have some time to make the necessary calls to report the error such as
// status update.
func WithTimeout(duration time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.timeout = duration
	}
}

// WithPollInterval specifies how long the Reconciler should wait before queueing
// a new reconciliation after a successful reconcile. The Reconciler requeues
// after a specified duration when it is not actively waiting for an external
// operation, but wishes to check whether an existing external resource needs to
// be synced to its Crossplane Managed resource.
func WithPollInterval(after time.Duration) ReconcilerOption {
	return func(r *Reconciler) {
		r.pollInterval = after
	}
}

// WithExternalConnecter specifies how the Reconciler should connect to the API
// used to sync and delete external resources.
func WithExternalConnecter(c ExternalConnecter) ReconcilerOption {
	return func(r *Reconciler) {
		r.external.ExternalConnecter = c
	}
}

// WithConnectionPublishers specifies how the Reconciler should publish
// its connection details such as credentials and endpoints.
func WithConnectionPublishers(p ...ConnectionPublisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ConnectionPublisher = PublisherChain(p)
	}
}

// WithInitializers specifies how the Reconciler should initialize a
// managed resource before calling any of the ExternalClient functions.
func WithInitializers(i ...Initializer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.Initializer = InitializerChain(i)
	}
}

// WithFinalizer specifies how the Reconciler should add and remove
// finalizers to and from the managed resource.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.Finalizer = f
	}
}

// WithReferenceResolver specifies how the Reconciler should resolve any
// inter-resource references it encounters while reconciling managed resources.
func WithReferenceResolver(rr ReferenceResolver) ReconcilerOption {
	return func(r *Reconciler) {
		r.managed.ReferenceResolver = rr
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
		client:       m.GetClient(),
		newManaged:   nm,
		pollInterval: defaultpollInterval,
		timeout:      reconcileTimeout,
		managed:      defaultMRManaged(m),
		external:     defaultMRExternal(),
		log:          logging.NewNopLogger(),
		record:       event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a managed resource with an external resource.
func (r *Reconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// NOTE(negz): This method is a well over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout+reconcileGracePeriod)
	defer cancel()

	// Govet linter has a check for lost cancel funcs but it's a false positive
	// for child contexts as because parent's cancel is called, so we skip it
	// for this line.
	externalCtx, _ := context.WithTimeout(ctx, r.timeout) // nolint:govet

	managed := r.newManaged()
	if err := r.client.Get(ctx, req.NamespacedName, managed); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get managed resource", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetManaged)
	}

	record := r.record.WithAnnotations("external-name", meta.GetExternalName(managed))
	log = log.WithValues(
		"uid", managed.GetUID(),
		"version", managed.GetResourceVersion(),
		"external-name", meta.GetExternalName(managed),
	)

	// If managed resource has a deletion timestamp and and a deletion policy of
	// Orphan, we do not need to observe the external resource before attempting
	// to unpublish connection details and remove finalizer.
	if meta.WasDeleted(managed) && managed.GetDeletionPolicy() == xpv1.DeletionOrphan {
		log = log.WithValues("deletion-timestamp", managed.GetDeletionTimestamp())
		managed.SetConditions(xpv1.Deleting())

		// Empty ConnectionDetails are passed to UnpublishConnection because we
		// have not retrieved them from the external resource. In practice we
		// currently only write connection details to a Secret, and we rely on
		// garbage collection to delete the entire secret, regardless of the
		// supplied connection details.
		if err := r.managed.UnpublishConnection(ctx, managed, ConnectionDetails{}); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot unpublish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotUnpublish, err))
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.RemoveFinalizer(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot remove managed resource finalizer", "error", err)
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully unpublished our managed resource's connection
		// details and removed our finalizer. If we assume we were the only
		// controller that added a finalizer to this resource then it should no
		// longer exist and thus there is no point trying to update its status.
		log.Debug("Successfully deleted managed resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.managed.Initialize(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot initialize managed resource", "error", err)
		record.Event(managed, event.Warning(reasonCannotInitialize, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
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
			// If any of our referenced resources are not yet ready (or if we
			// encountered an error resolving them) we want to try again. If
			// this is the first time we encounter this situation we'll be
			// requeued implicitly due to the status update. If not, we want
			// requeue explicitly, which will trigger backoff.
			log.Debug("Cannot resolve managed resource references", "error", err)
			record.Event(managed, event.Warning(reasonCannotResolveRefs, err))
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
	}

	external, err := r.external.Connect(externalCtx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider or its secret are missing
		// or invalid. If this is first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we requeue explicitly, which will trigger
		// backoff.
		log.Debug("Cannot connect to provider", "error", err)
		record.Event(managed, event.Warning(reasonCannotConnect, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileConnect)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	observation, err := external.Observe(externalCtx, managed)
	if err != nil {
		// We'll usually hit this case if our Provider credentials are invalid
		// or insufficient for observing the external resource type we're
		// concerned with. If this is the first time we encounter this issue
		// we'll be requeued implicitly when we update our status with the new
		// error condition. If not, we requeue explicitly, which will
		// trigger backoff.
		log.Debug("Cannot observe external resource", "error", err)
		record.Event(managed, event.Warning(reasonCannotObserve, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileObserve)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if meta.WasDeleted(managed) {
		log = log.WithValues("deletion-timestamp", managed.GetDeletionTimestamp())
		managed.SetConditions(xpv1.Deleting())

		// We'll only reach this point if deletion policy is not orphan, so we
		// are safe to call external deletion if external resource exists.
		if observation.ResourceExists {
			if err := external.Delete(externalCtx, managed); err != nil {
				// We'll hit this condition if we can't delete our external
				// resource, for example if our provider credentials don't have
				// access to delete it. If this is the first time we encounter
				// this issue we'll be requeued implicitly when we update our
				// status with the new error condition. If not, we want requeue
				// explicitly, which will trigger backoff.
				log.Debug("Cannot delete external resource", "error", err)
				record.Event(managed, event.Warning(reasonCannotDelete, err))
				managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileDelete)))
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
			}

			// We've successfully requested deletion of our external resource.
			// We queue another reconcile after a short wait rather than
			// immediately finalizing our delete in order to verify that the
			// external resource was actually deleted. If it no longer exists
			// we'll skip this block on the next reconcile and proceed to
			// unpublish and finalize. If it still exists we'll re-enter this
			// block and try again.
			log.Debug("Successfully requested deletion of external resource")
			record.Event(managed, event.Normal(reasonDeleted, "Successfully requested deletion of external resource"))
			managed.SetConditions(xpv1.ReconcileSuccess())
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.UnpublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot unpublish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotUnpublish, err))
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
		if err := r.managed.RemoveFinalizer(ctx, managed); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger
			// backoff.
			log.Debug("Cannot remove managed resource finalizer", "error", err)
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully deleted our external resource (if necessary) and
		// removed our finalizer. If we assume we were the only controller that
		// added a finalizer to this resource then it should no longer exist and
		// thus there is no point trying to update its status.
		log.Debug("Successfully deleted managed resource")
		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.managed.PublishConnection(ctx, managed, observation.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot publish connection details", "error", err)
		record.Event(managed, event.Warning(reasonCannotPublish, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.AddFinalizer(ctx, managed); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot add finalizer", "error", err)
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if !observation.ResourceExists {
		managed.SetConditions(xpv1.Creating())
		creation, err := external.Create(externalCtx, managed)
		if err != nil {
			// We'll hit this condition if we can't create our external
			// resource, for example if our provider credentials don't have
			// access to create it. If this is the first time we encounter this
			// issue we'll be requeued implicitly when we update our status with
			// the new error condition. If not, we requeue explicitly, which will trigger backoff.
			log.Debug("Cannot create external resource", "error", err)
			record.Event(managed, event.Warning(reasonCannotCreate, err))
			managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileCreate)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		if creation.ExternalNameAssigned {
			en := meta.GetExternalName(managed)
			// We will retry in all cases where the error comes from the api-server.
			// At one point, context deadline will be exceeded and we'll get out
			// of the loop. In that case, we warn the user that the external resource
			// might be leaked.
			err := retry.OnError(retry.DefaultRetry, resource.IsAPIError, func() error {
				nn := types.NamespacedName{Name: managed.GetName()}
				if err := r.client.Get(ctx, nn, managed); err != nil {
					return err
				}
				meta.SetExternalName(managed, en)
				return r.client.Update(ctx, managed)
			})
			if err != nil {
				log.Debug("Cannot update managed resource", "error", err)
				record.Event(managed, event.Warning(reasonCannotUpdateManaged, errors.Wrap(err, errUpdateManagedAfterCreate)))
				managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errUpdateManagedAfterCreate)))
				return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
			}
		}

		if err := r.managed.PublishConnection(ctx, managed, creation.ConnectionDetails); err != nil {
			// If this is the first time we encounter this issue we'll be
			// requeued implicitly when we update our status with the new error
			// condition. If not, we requeue explicitly, which will trigger backoff.
			log.Debug("Cannot publish connection details", "error", err)
			record.Event(managed, event.Warning(reasonCannotPublish, err))
			managed.SetConditions(xpv1.ReconcileError(err))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}

		// We've successfully created our external resource. In many cases the
		// creation process takes a little time to finish. We requeue explicitly
		// order to observe the external resource to determine whether it's
		// ready for use.
		log.Debug("Successfully requested creation of external resource")
		record.Event(managed, event.Normal(reasonCreated, "Successfully requested creation of external resource"))
		managed.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if observation.ResourceLateInitialized {
		// Note that this update may reset any pending updates to the status of
		// the managed resource from when it was observed above. This is because
		// the API server replies to the update with its unchanged view of the
		// resource's status, which is subsequently deserialized into managed.
		// This is usually tolerable because the update will implicitly requeue
		// an immediate reconcile which should re-observe the external resource
		// and persist its status.
		if err := r.client.Update(ctx, managed); err != nil {
			log.Debug(errUpdateManaged, "error", err)
			record.Event(managed, event.Warning(reasonCannotUpdateManaged, err))
			managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errUpdateManaged)))
			return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
		}
	}

	if observation.ResourceUpToDate {
		// We did not need to create, update, or delete our external resource.
		// Per the below issue nothing will notify us if and when the external
		// resource we manage changes, so we requeue a speculative reconcile
		// after the specified poll interval in order to observe it and react
		// accordingly.
		// https://github.com/crossplane/crossplane/issues/289
		log.Debug("External resource is up to date", "requeue-after", time.Now().Add(r.pollInterval))
		managed.SetConditions(xpv1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	update, err := external.Update(externalCtx, managed)
	if err != nil {
		// We'll hit this condition if we can't update our external resource,
		// for example if our provider credentials don't have access to update
		// it. If this is the first time we encounter this issue we'll be
		// requeued implicitly when we update our status with the new error
		// condition. If not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot update external resource")
		record.Event(managed, event.Warning(reasonCannotUpdate, err))
		managed.SetConditions(xpv1.ReconcileError(errors.Wrap(err, errReconcileUpdate)))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	if err := r.managed.PublishConnection(ctx, managed, update.ConnectionDetails); err != nil {
		// If this is the first time we encounter this issue we'll be requeued
		// implicitly when we update our status with the new error condition. If
		// not, we requeue explicitly, which will trigger backoff.
		log.Debug("Cannot publish connection details", "error", err)
		record.Event(managed, event.Warning(reasonCannotPublish, err))
		managed.SetConditions(xpv1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
	}

	// We've successfully updated our external resource. Per the below issue
	// nothing will notify us if and when the external resource we manage
	// changes, so we requeue a speculative reconcile after the specified poll
	// interval in order to observe it and react accordingly.
	// https://github.com/crossplane/crossplane/issues/289
	log.Debug("Successfully requested update of external resource", "requeue-after", time.Now().Add(r.pollInterval))
	record.Event(managed, event.Normal(reasonUpdated, "Successfully requested update of external resource"))
	managed.SetConditions(xpv1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: r.pollInterval}, errors.Wrap(r.client.Status().Update(ctx, managed), errUpdateManagedStatus)
}
