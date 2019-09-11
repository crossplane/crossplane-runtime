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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
)

const (
	controllerNameDefaultClass   = "defaultclass.crossplane.io"
	defaultClassWait             = 1 * time.Minute
	defaultClassReconcileTimeout = 1 * time.Minute
)

// Label values for listing default portable classes
const (
	LabelKeyDefaultClass = "default"
	LabelValueTrue       = "true"
)

// Error strings
const (
	errFailedList              = "unable to list portable classes for claim kind"
	errNoPortableClass         = "unable to locate a portable class that specifies a default class for claim kind"
	errMultiplePortableClasses = "multiple portable classes that specify a default class defined for claim kind"
)

// A PortableClassKind contains the type metadata for a kind of portable class.
type PortableClassKind struct {
	Singular schema.GroupVersionKind
	Plural   schema.GroupVersionKind
}

// DefaultClassReconciler reconciles resource claims to the
// default resource class for their given kind according to existing
// portable classes. Predicates ensure that only claims with no resource class
// reference are reconciled.
type DefaultClassReconciler struct {
	client               client.Client
	converter            runtime.ObjectConvertor
	labels               map[string]string
	newClaim             func() Claim
	newPortableClass     func() PortableClass
	newPortableClassList func() PortableClassList
}

// A DefaultClassReconcilerOption configures a DefaultClassReconciler.
type DefaultClassReconcilerOption func(*DefaultClassReconciler)

// WithObjectConverter specifies how the DefaultClassReconciler should convert
// an *UnstructuredList into a concrete list type.
func WithObjectConverter(oc runtime.ObjectConvertor) DefaultClassReconcilerOption {
	return func(r *DefaultClassReconciler) {
		r.converter = oc
	}
}

// WithLabels specifies how the DefaultClassReconciler should search
// for a default class
func WithLabels(labels map[string]string) DefaultClassReconcilerOption {
	return func(r *DefaultClassReconciler) {
		r.labels = labels
	}
}

// NewDefaultClassReconciler creates a new DefaultReconciler for the claim kind.
func NewDefaultClassReconciler(m manager.Manager, of ClaimKind, by PortableClassKind, o ...DefaultClassReconcilerOption) *DefaultClassReconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }
	np := func() PortableClass { return MustCreateObject(by.Singular, m.GetScheme()).(PortableClass) }
	npl := func() PortableClassList { return MustCreateObject(by.Plural, m.GetScheme()).(PortableClassList) }

	// Panic early if we've been asked to reconcile a claim, portable class, or portable class list that has
	// not been registered with our controller manager's scheme.
	_, _, _ = nc(), np(), npl()

	labels := map[string]string{LabelKeyDefaultClass: LabelValueTrue}

	r := &DefaultClassReconciler{
		client:               m.GetClient(),
		converter:            m.GetScheme(),
		labels:               labels,
		newClaim:             nc,
		newPortableClass:     np,
		newPortableClassList: npl,
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

// Reconcile reconciles a claim to the default class reference for its kind.
func (r *DefaultClassReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("Reconciling", "request", req, "controller", controllerNameDefaultClass)

	ctx, cancel := context.WithTimeout(context.Background(), defaultClassReconcileTimeout)
	defer cancel()

	claim := r.newClaim()
	if err := r.client.Get(ctx, req.NamespacedName, claim); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(err), errGetClaim)
	}

	// Get portable classes for claim kind in claim's namespace
	portables := r.newPortableClassList()
	if err := r.client.List(ctx, portables, client.InNamespace(req.Namespace), client.MatchingLabels(r.labels)); err != nil {
		// If this is the first time we encounter listing error we'll be
		// requeued implicitly due to the status update. If not, we don't
		// care to requeue because list parameters will not change.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errFailedList)))
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	items := portables.GetPortableClassItems()
	// Check to see if no defaults defined for claim kind.
	if len(items) == 0 {
		// If this is the first time we encounter no default portable classes we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if a default portable class has been created.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errNoPortableClass)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Check to see if multiple defaults defined for claim kind.
	if len(items) > 1 {
		// If this is the first time we encounter multiple default portable classes we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if only one default portable class exists.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errMultiplePortableClasses)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Set portable class reference on claim to default portable class.
	portable := items[0]
	claim.SetPortableClassReference(&corev1.LocalObjectReference{Name: portable.GetName()})

	// Do not requeue, claim controller will see update and claim
	// with class reference set will pass predicates.
	return reconcile.Result{Requeue: false}, errors.Wrap(IgnoreNotFound(r.client.Update(ctx, claim)), errUpdateClaimStatus)
}
