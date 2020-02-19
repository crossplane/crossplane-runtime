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

package secret

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const secretReconcileTimeout = 1 * time.Minute

// Error messages.
const (
	errGetSecret         = "cannot get managed resource's connection secret"
	errUpdateSecret      = "cannot update connection secret"
	errUnexpectedFromUID = "unexpected propagate from uid on propagated secret"
	errUnexpectedToUID   = "unexpected propagate to uid on propagator secret"
)

// Event reasons
const (
	reasonPropagatedFrom event.Reason = "PropagatedDataFrom"
	reasonPropagatedTo   event.Reason = "PropagatedDataTo"
)

// ControllerName returns the recommended name for controllers that use this
// package to reconcile a particular kind of resource claim.
func ControllerName(kind string) string {
	return "secretpropagating/" + strings.ToLower(kind)
}

// Reconciler reconciles secrets by propagating their data from another secret.
// Both secrets must consent to this process by including propagation
// annotations. The Reconciler assumes it has a watch on both propagating (from)
// and propagated (to) secrets.
type Reconciler struct {
	client client.Client

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

// NewReconciler returns a Reconciler that reconciles secrets by propagating
// their data from another secret. Both secrets must consent to this process by
// including propagation annotations. The Reconciler assumes it has a watch on
// both propagating (from) and propagated (to) secrets.
func NewReconciler(m manager.Manager, o ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: m.GetClient(),
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile a secret by propagating its data from another secret.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), secretReconcileTimeout)
	defer cancel()

	// The 'to' secret is also known as the 'propagated' secret. We guard
	// against abusers of the propagation process by requiring that both
	// secrets consent to propagation by specifying each other's UID. We
	// cannot know the UID of a secret that doesn't exist, so the propagated
	// secret must be created outside of the propagation process.
	to := &corev1.Secret{}
	if err := r.client.Get(ctx, req.NamespacedName, to); err != nil {
		// There's no propagation to be done if the secret we propagate to
		// does not exist. We assume we have a watch on that secret and will
		// be queued if/when it is created. Otherwise we'll be requeued
		// implicitly because we return an error.
		log.Debug("Cannot get propagated secret", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetSecret)
	}

	record := r.record.WithAnnotations("to-namespace", to.GetNamespace(), "to-name", to.GetName())
	log = log.WithValues("to-namespace", to.GetNamespace(), "to-name", to.GetName())

	// The 'from' secret is also know as the 'propagating' secret.
	from := &corev1.Secret{}
	n := types.NamespacedName{
		Namespace: to.GetAnnotations()[resource.AnnotationKeyPropagateFromNamespace],
		Name:      to.GetAnnotations()[resource.AnnotationKeyPropagateFromName],
	}
	if err := r.client.Get(ctx, n, from); err != nil {
		// There's no propagation to be done if the secret we're propagating
		// from does not exist. We assume we have a watch on that secret and
		// will be queued if/when it is created. Otherwise we'll be requeued
		// implicitly because we return an error.
		log.Debug("Cannot get propagating secret", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetSecret)
	}

	record = record.WithAnnotations("from-namespace", from.GetNamespace(), "from-name", from.GetName())
	log = log.WithValues("from-namespace", from.GetNamespace(), "from-name", from.GetName())

	if a := to.GetAnnotations()[resource.AnnotationKeyPropagateFromUID]; a != string(from.GetUID()) {
		// The propagated secret expected a different propagating secret. We
		// assume we have a watch on both secrets, and will be requeued if
		// and when this situation is remedied.
		log.Debug("Unexpected propagate-from UID", "want", a, "got", string(from.GetUID()))
		return reconcile.Result{}, errors.New(errUnexpectedFromUID)
	}

	if a, ok := from.GetAnnotations()[strings.Join([]string{resource.AnnotationKeyPropagateToPrefix, string(to.GetUID())}, resource.AnnotationDelimiter)]; !ok {
		// The propagating secret did not expect this propagated secret. We
		// assume we have a watch on both secrets, and will be requeued if and
		// when this situation is remedied.
		log.Debug("Unexpected propagate-to UID", "want", a)
		return reconcile.Result{}, errors.New(errUnexpectedToUID)
	}

	to.Data = from.Data

	// If our update was unsuccessful. Keep trying to update
	// additional secrets but implicitly requeue when finished.
	log.Debug("Propagated secret data")
	record.Event(to, event.Normal(reasonPropagatedFrom, "Data propagated from secret"))
	record.Event(from, event.Normal(reasonPropagatedTo, "Data propagated to secret"))
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Update(ctx, to), errUpdateSecret)
}
