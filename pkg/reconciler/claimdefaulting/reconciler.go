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

package claimdefaulting

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	claimDefaultingReconcileTimeout     = 1 * time.Minute
	claimDefaultingReconcileMaxJitterMs = 1500

	aShortWait = 30 * time.Second
)

// Error strings.
const (
	errGetClaim    = "cannot get resource claim"
	errUpdateClaim = "cannot update resource claim"
	errListClasses = "cannot list resource classes"
)

// Event reasons.
const reasonClassFound = "DefaultResourceClass"

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of resource claim.
func ControllerName(kind string) string {
	return "claimdefaulting/" + strings.ToLower(kind)
}

// A Jitterer sleeps for a random amount of time in order to decrease the chance
// of any one controller predictably winning the race to schedule claims to a
// class, for example because it has fewer classes to list and select from than
// its competitors.
type Jitterer func()

// A Reconciler reconciles resource claims by setting their class reference to
// the resource class annotated as the default. If multiple classes are
// annotated as the default one of the annotated classes will be set, but which
// one is undefined.
type Reconciler struct {
	client    client.Client
	newClaim  func() resource.Claim
	classKind resource.ClassKind
	jitter    Jitterer

	log    logging.Logger
	record event.Recorder
}

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption func(*Reconciler)

// WithDefaultingJitterer specifies the Jitterer a Reconciler should use.
func WithDefaultingJitterer(j Jitterer) ReconcilerOption {
	return func(r *Reconciler) {
		r.jitter = j
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

// NewReconciler returns a Reconciler that sets the class reference of a
// resource claim to the resource class annotated as the default.
func NewReconciler(m manager.Manager, of resource.ClaimKind, to resource.ClassKind, o ...ReconcilerOption) *Reconciler {
	nc := func() resource.Claim {
		return resource.MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(resource.Claim)
	}

	// Panic early if we've been asked to reconcile a claim or resource kind
	// that has not been registered with our controller manager's scheme.
	_ = nc()

	r := &Reconciler{
		client:    m.GetClient(),
		newClaim:  nc,
		classKind: to,
		jitter: func() {
			random := rand.New(rand.NewSource(time.Now().UnixNano()))
			time.Sleep(time.Duration(random.Intn(claimDefaultingReconcileMaxJitterMs)) * time.Millisecond)
		},
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource claim by using its class selector to select and allocate
// it a resource class.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), claimDefaultingReconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug("Cannot get resource claim", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetClaim)
	}

	record := r.record.WithAnnotations(
		"external-name", meta.GetExternalName(claim),
		"class-kind", r.classKind.Kind,
	)
	log = log.WithValues(
		"uid", claim.GetUID(),
		"version", claim.GetResourceVersion(),
		"external-name", meta.GetExternalName(claim),
		"class-kind", r.classKind.Kind,
	)

	// There could be several controllers racing to schedule this claim. If it
	// was scheduled since we were queued then another controller won and we
	// should abort.
	if claim.GetClassReference() != nil {
		log.Debug("Resource class is already set")
		return reconcile.Result{Requeue: false}, nil
	}

	classes := &unstructured.UnstructuredList{}
	classes.SetGroupVersionKind(r.classKind.List())

	if err := r.client.List(ctx, classes); err != nil {
		// Claim defaulting controllers don't update the synced status because
		// no one scheduler has the full view of whether the process failed or
		// succeeded. It's possible another controller can successfully set a
		// class even though we can't, so it would be confusing to mark this
		// claim as failing to be reconciled. Instead we return an error - we'll
		// be requeued but abort immediately if the claim was defaulted.
		log.Debug("Cannot list resource classes", "error", err)
		return reconcile.Result{}, errors.Wrap(err, errListClasses)
	}

	defaults := []unstructured.Unstructured{}
	for _, c := range classes.Items {
		if c.GetAnnotations()[v1alpha1.AnnotationDefaultClassKey] == v1alpha1.AnnotationDefaultClassValue {
			defaults = append(defaults, c)
		}
	}

	if len(defaults) == 0 {
		// None of our classes are annotated as the default. We can't be sure
		// whether another controller owns the default class, or whether there
		// is no default class, so we requeue after a short wait. We'll abort
		// the next reconcile immediately if another controller defaulted the
		// claim.
		log.Debug("No default resource classes found", "requeue-after", time.Now().Add(aShortWait))
		return reconcile.Result{RequeueAfter: aShortWait}, nil
	}

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected := defaults[random.Intn(len(defaults))]
	claim.SetClassReference(meta.ReferenceTo(&selected, schema.GroupVersionKind(r.classKind)))

	// There could be several controllers racing to default this claim to a
	// class. We sleep for a randomly jittered amount of time before trying to
	// update the class reference to decrease the chance of any one controller
	// predictably winning the race, for example because it has fewer classes to
	// list and select from than its competitors.
	r.jitter()

	// Attempt to set the class reference. If a competing controller beat us
	// we'll fail the write because the claim's resource version has changed
	// since we read it. We'll be requeued, but will abort immediately if the
	// claim was defaulted.
	log.Debug("Attempting to set resource class", "class-name", selected.GetName())
	record.Event(claim, event.Normal(reasonClassFound, "Selected default resource class", "class-name", selected.GetName()))
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Update(ctx, claim), errUpdateClaim)
}
