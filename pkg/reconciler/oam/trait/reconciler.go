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

package trait

import (
	"context"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	reconcileTimeout = 1 * time.Minute
	shortWait        = 30 * time.Second
	longWait         = 1 * time.Minute
)

// Reconcile error strings.
const (
	errGetTrait               = "cannot get trait"
	errUpdateTraitStatus      = "cannot update trait status"
	errTraitModify            = "cannot apply trait modification"
	errGetTranslation         = "cannot get translation for workload reference in trait"
	errApplyTraitModification = "cannot apply trait modification to workload translation"
)

// Reconcile event reasons.
const (
	reasonTraitWait   = "WaitingForWorkloadTranslation"
	reasonTraitModify = "PackageModified"

	reasonCannotGetTranslation    = "CannotGetReferencedWorkloadTranslation"
	reasonCannotModifyTranslation = "CannotModifyTranslation"
	reasonCannotApplyModification = "CannotApplyModification"
)

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

// WithModifier specifies how the Reconciler should modify the workload translation.
func WithModifier(m Modifier) ReconcilerOption {
	return func(r *Reconciler) {
		r.trait = m
	}
}

// WithApplicator specifies how the Reconciler should apply the workload
// translation modification.
func WithApplicator(a resource.Applicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.applicator = a
	}
}

// A Reconciler reconciles OAM traits by modifying the object that a workload
// has been translated into.
type Reconciler struct {
	client         client.Client
	newTrait       func() resource.Trait
	newTranslation func() resource.Object
	trait          Modifier
	applicator     resource.Applicator

	log    logging.Logger
	record event.Recorder
}

// NewReconciler returns a Reconciler that reconciles OAM traits by fetching
// their referenced workload's translation and applying modifications.
func NewReconciler(m ctrl.Manager, trait resource.TraitKind, trans resource.ObjectKind, o ...ReconcilerOption) *Reconciler {
	nt := func() resource.Trait {
		return resource.MustCreateObject(schema.GroupVersionKind(trait), m.GetScheme()).(resource.Trait)
	}

	nr := func() resource.Object {
		return resource.MustCreateObject(schema.GroupVersionKind(trans), m.GetScheme()).(resource.Object)
	}

	r := &Reconciler{
		client:         m.GetClient(),
		newTrait:       nt,
		newTranslation: nr,
		trait:          ModifyFn(NoopModifier),
		applicator:     resource.ApplyFn(resource.Apply),

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile an OAM trait type by modifying its referenced workload's
// KubernetesApplication.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	trait := r.newTrait()
	if err := r.client.Get(ctx, req.NamespacedName, trait); err != nil {
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetTrait)
	}

	log = log.WithValues("uid", trait.GetUID(), "version", trait.GetResourceVersion())

	translation := r.newTranslation()

	// TODO(hasheddan): we make the assumption here that the workload
	// translation object that we are modifying has the same name as the
	// workload itself. This would not work if a translation produced multiple
	// objects of the same kind as they would not be permitted to have the same
	// name.
	err := r.client.Get(ctx, types.NamespacedName{Name: trait.GetWorkloadReference().Name, Namespace: trait.GetNamespace()}, translation)
	if kerrors.IsNotFound(err) {
		log.Debug("Waiting for referenced workload's translation", "kind", trait.GetObjectKind().GroupVersionKind().String())
		r.record.Event(trait, event.Normal(reasonTraitWait, "Waiting for workload translation to exist"))
		trait.SetConditions(v1alpha1.ReconcileSuccess())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, trait), errUpdateTraitStatus)
	}
	if err != nil {
		log.Debug("Cannot get workload translation", "error", err, "requeue-after", time.Now().Add(shortWait))
		r.record.Event(trait, event.Warning(reasonCannotGetTranslation, err))
		trait.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errGetTranslation)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, trait), errUpdateTraitStatus)
	}

	if err := r.trait.Modify(ctx, translation, trait); err != nil {
		log.Debug("Cannot modify workload translation", "error", err, "requeue-after", time.Now().Add(shortWait))
		r.record.Event(trait, event.Warning(reasonCannotModifyTranslation, err))
		trait.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errTraitModify)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, trait), errUpdateTraitStatus)
	}

	// The trait's referenced workload should always be translated in an
	// object(s) that is controlled by the workload. In the case where an
	// object(s) already exists in the same namespace and with the same name
	// before it is created, this wll guard against modifying it.
	if err := r.applicator.Apply(ctx, r.client, translation, resource.ControllersMustMatch()); err != nil {
		log.Debug("Cannot apply workload translation", "error", err, "requeue-after", time.Now().Add(shortWait))
		r.record.Event(trait, event.Warning(reasonCannotApplyModification, err))
		trait.SetConditions(v1alpha1.ReconcileError(errors.Wrap(err, errApplyTraitModification)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, trait), errUpdateTraitStatus)
	}

	r.record.Event(trait, event.Normal(reasonTraitModify, "Successfully modifed workload translation"))
	log.Debug("Successfully modified referenced workload", "kind", trait.GetObjectKind().GroupVersionKind().String())

	trait.SetConditions(v1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, trait), errUpdateTraitStatus)
}
