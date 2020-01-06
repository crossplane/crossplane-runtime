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
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
)

// Supported resources with all of these annotations will be fully or partially
// propagated to the named resource of the same kind, assuming it exists and
// consents to propagation.
const (
	AnnotationKeyPropagateToPrefix = "to.propagate.crossplane.io"
	SlashDelimeter                 = "/"

	AnnotationKeyPropagateFromNamespace = "crossplane.io/propagate-from-namespace"
	AnnotationKeyPropagateFromName      = "crossplane.io/propagate-from-name"
	AnnotationKeyPropagateFromUID       = "crossplane.io/propagate-from-uid"
)

type annotated interface {
	GetAnnotations() map[string]string
}

const (
	secretControllerName   = "secretpropagator.crossplane.io"
	secretReconcileTimeout = 1 * time.Minute
	errUnexpectedFromUID   = "unexpected propagate from uid on propagated secret"
	errUnexpectedToUID     = "unexpected propagate to uid on propagator secret"
)

// NewSecretPropagatingReconciler returns a Reconciler that reconciles secrets
// by propagating their data to another secret. Both secrets must consent to
// this process by including propagation annotations. The Reconciler assumes it
// has a watch on both propagating (from) and propagated (to) secrets.
func NewSecretPropagatingReconciler(m manager.Manager) reconcile.Reconciler { // nolint:gocyclo
	// NOTE(hasheddan): This function is over our cyclomatic complexity goal.
	// Be wary of adding additional complexity.
	client := m.GetClient()

	return reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		log.V(logging.Debug).Info("Reconciling", "controller", secretControllerName, "request", req)

		ctx, cancel := context.WithTimeout(context.Background(), secretReconcileTimeout)
		defer cancel()

		// The 'to' secret is also known as the 'propagated' secret. We guard
		// against abusers of the propagation process by requiring that both
		// secrets consent to propagation by specifying each other's UID. We
		// cannot know the UID of a secret that doesn't exist, so the propagated
		// secret must be created outside of the propagation process.
		to := &corev1.Secret{}
		if err := client.Get(ctx, req.NamespacedName, to); err != nil {
			// There's no propagation to be done if the secret we propagate to
			// does not exist. We assume we have a watch on that secret and will
			// be queued if/when it is created. Otherwise we'll be requeued
			// implicitly because we return an error.
			return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetSecret)
		}
		// The 'from' secret is also know as the 'propagating' secret.
		from := &corev1.Secret{}
		n := types.NamespacedName{
			Namespace: to.GetAnnotations()[AnnotationKeyPropagateFromNamespace],
			Name:      to.GetAnnotations()[AnnotationKeyPropagateFromName],
		}
		if err := client.Get(ctx, n, from); err != nil {
			// There's no propagation to be done if the secret we're propagating
			// from does not exist. We assume we have a watch on that secret and
			// will be queued if/when it is created. Otherwise we'll be requeued
			// implicitly because we return an error.
			return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetSecret)
		}

		if to.GetAnnotations()[AnnotationKeyPropagateFromUID] != string(from.GetUID()) {
			// The propagated secret expected a different propagating secret. We
			// assume we have a watch on both secrets, and will be requeued if
			// and when this situation is remedied.
			return reconcile.Result{}, errors.New(errUnexpectedFromUID)
		}

		if _, ok := from.GetAnnotations()[strings.Join([]string{AnnotationKeyPropagateToPrefix, string(to.GetUID())}, SlashDelimeter)]; !ok {
			// The propagating secret expected a different propagated secret. We
			// assume we have a watch on both secrets, and will be requeued if
			// and when this situation is remedied.
			return reconcile.Result{}, errors.New(errUnexpectedToUID)
		}

		to.Data = from.Data

		// If our update was unsuccessful. Keep trying to update
		// additional secrets but implicitly requeue when finished.
		return reconcile.Result{Requeue: false}, errors.Wrap(client.Update(ctx, to), errUpdateSecret)
	})
}
