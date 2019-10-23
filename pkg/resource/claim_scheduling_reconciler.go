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
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
)

const (
	claimSchedulingControllerName       = "resourceclaimscheduler.crossplane.io"
	claimSchedulingReconcileTimeout     = 1 * time.Minute
	claimSchedulingReconcileMaxJitterMs = 1500
)

const (
	errListClasses = "cannot list resource classes"
)

// A Jitterer sleeps for a random amount of time in order to decrease the chance
// of any one controller predictably winning the race to schedule claims to a
// class, for example because it has fewer classes to list and select from than
// its competitors.
type Jitterer func()

// A ClaimSchedulingReconciler schedules resource claims to a resource class
// that matches their class selector. Claims are reconciled by randomly
// selecting a matching resource class and attempting to set it as the claim's
// class reference. The ClaimSchedulingReconciler is designed for use in
// claim scheduling controllers that race several others to schedule a claim.
type ClaimSchedulingReconciler struct {
	client    client.Client
	newClaim  func() Claim
	classKind ClassKind
	jitter    Jitterer
}

// A ClaimSchedulingReconcilerOption configures a ClaimSchedulingReconciler.
type ClaimSchedulingReconcilerOption func(*ClaimSchedulingReconciler)

// WithSchedulingJitterer specifies the Jitterer a ClaimSchedulingReconciler should use.
func WithSchedulingJitterer(j Jitterer) ClaimSchedulingReconcilerOption {
	return func(r *ClaimSchedulingReconciler) {
		r.jitter = j
	}
}

// NewClaimSchedulingReconciler returns a ClaimSchedulingReconciler that
// schedules resource claims to a resource class that matches their class
// selector.
func NewClaimSchedulingReconciler(m manager.Manager, of ClaimKind, to ClassKind, o ...ClaimSchedulingReconcilerOption) *ClaimSchedulingReconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }

	// Panic early if we've been asked to reconcile a claim or resource kind
	// that has not been registered with our controller manager's scheme.
	_ = nc()

	r := &ClaimSchedulingReconciler{
		client:    m.GetClient(),
		newClaim:  nc,
		classKind: to,
		jitter: func() {
			random := rand.New(rand.NewSource(time.Now().UnixNano()))
			time.Sleep(time.Duration(random.Intn(claimSchedulingReconcileMaxJitterMs)) * time.Millisecond)
		},
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a resource claim by using its class selector to select and allocate
// it a resource class.
func (r *ClaimSchedulingReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("Reconciling", "controller", claimSchedulingControllerName, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), claimSchedulingReconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetClaim)
	}

	// There could be several controllers racing to schedule this claim. If it
	// was scheduled since we were queued then another controller won and we
	// should abort.
	if claim.GetClassReference() != nil {
		return reconcile.Result{Requeue: false}, nil
	}

	classes := &unstructured.UnstructuredList{}
	classes.SetGroupVersionKind(r.classKind.List())

	if err := r.client.List(ctx, classes, client.MatchingLabels(claim.GetClassSelector().MatchLabels)); err != nil {
		// Claim scheduler controllers don't update the synced status because
		// no one scheduler has the full view of whether the process failed or
		// succeeded. It's possible another controller can successfully set a
		// class even though we can't, so it would be confusing to mark this
		// claim as failing to be reconciled. Instead we return an error - we'll
		// be requeued but abort immediately if the claim was scheduled.
		return reconcile.Result{}, errors.Wrap(err, errListClasses)
	}

	if len(classes.Items) == 0 {
		// None of our classes matched the class selector.
		// There's nothing for us to do.
		return reconcile.Result{Requeue: false}, nil
	}

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected := classes.Items[random.Intn(len(classes.Items))]
	claim.SetClassReference(meta.ReferenceTo(&selected, schema.GroupVersionKind(r.classKind)))

	// There could be several controllers racing to schedule this claim to a
	// class. We sleep for a randomly jittered amount of time before trying to
	// update the class reference to decrease the chance of any one controller
	// predictably winning the race, for example because it has fewer classes to
	// list and select from than its competitors.
	r.jitter()

	// Attempt to set the class reference. If a competing controller beat us
	// we'll fail the write because the claim's resource version has changed
	// since we read it. We'll be requeued, but will abort immediately if the
	// claim was scheduled.
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Update(ctx, claim), errUpdateClaim)
}
