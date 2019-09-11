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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// Error strings
const (
	errFailedList                    = "unable to list policies for claim kind"
	errFailedPortableClassConversion = "unable to convert located portable class to correct kind"
	errNoPortableClass               = "unable to locate a portable class that specifies a default class for claim kind"
	errMultiplePortableClasses       = "multiple portable classes that specify a default class defined for claim kind"
)

// A PortableClassKind contains the type metadata for a kind of portable class.
type PortableClassKind struct {
	Singular schema.GroupVersionKind
	Plural   schema.GroupVersionKind
}

// DefaultClassReconciler reconciles resource claims to the
// default resource class for their given kind according to existing
// policies. Predicates ensure that only claims with no resource class
// reference are reconciled.
type DefaultClassReconciler struct {
	client                client.Client
	converter             runtime.ObjectConvertor
	portableClassKind     schema.GroupVersionKind
	portableClassListKind schema.GroupVersionKind
	newClaim              func() Claim
	newPortableClass      func() PortableClass
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

// NewDefaultClassReconciler creates a new DefaultReconciler for the claim kind.
func NewDefaultClassReconciler(m manager.Manager, of ClaimKind, by PortableClassKind, o ...DefaultClassReconcilerOption) *DefaultClassReconciler {
	nc := func() Claim { return MustCreateObject(schema.GroupVersionKind(of), m.GetScheme()).(Claim) }
	np := func() PortableClass { return MustCreateObject(by.Singular, m.GetScheme()).(PortableClass) }
	npl := func() PortableClassList { return MustCreateObject(by.Plural, m.GetScheme()).(PortableClassList) }

	// Panic early if we've been asked to reconcile a claim, policy, or policy list that has
	// not been registered with our controller manager's scheme.
	_, _, _ = nc(), np(), npl()

	r := &DefaultClassReconciler{
		client:                m.GetClient(),
		converter:             m.GetScheme(),
		portableClassKind:     by.Singular,
		portableClassListKind: by.Plural,
		newClaim:              nc,
		newPortableClass:      np,
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

	// Get policies for claim kind in claim's namespace
	portables := &unstructured.UnstructuredList{}
	portables.SetGroupVersionKind(r.portableClassListKind)
	options := client.InNamespace(req.Namespace)
	if err := r.client.List(ctx, portables, options); err != nil {
		// If this is the first time we encounter listing error we'll be
		// requeued implicitly due to the status update. If not, we don't
		// care to requeue because list parameters will not change.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errFailedList)))
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Check to see if no defaults defined for claim kind.
	if len(portables.Items) == 0 {
		// If this is the first time we encounter no policies we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if a policy has been created.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errNoPortableClass)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Check to see if multiple policies defined for claim kind.
	if len(portables.Items) > 1 {
		// If this is the first time we encounter multiple policies we'll be
		// requeued implicitly due to the status update. If not, we will requeue
		// after a time to see if only one policy class exists.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errMultiplePortableClasses)))
		return reconcile.Result{RequeueAfter: defaultClassWait}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Make sure single item is of correct policy kind.
	portable := r.newPortableClass()
	p := portables.Items[0]
	p.SetGroupVersionKind(r.portableClassKind)
	if err := r.converter.Convert(&p, portable, ctx); err != nil {
		// If this is the first time we encounter conversion error we'll be
		// requeued implicitly due to the status update. If not, we don't
		// care to requeue because conversion will likely not change.
		claim.SetConditions(corev1alpha1.ReconcileError(errors.New(errFailedPortableClassConversion)))
		return reconcile.Result{}, errors.Wrap(IgnoreNotFound(r.client.Status().Update(ctx, claim)), errUpdateClaimStatus)
	}

	// Set class reference on claim to default resource class.
	claim.SetClassReference(portable.GetClassReference())

	// Do not requeue, claim controller will see update and claim
	// with class reference set will pass predicates.
	return reconcile.Result{Requeue: false}, errors.Wrap(IgnoreNotFound(r.client.Update(ctx, claim)), errUpdateClaimStatus)
}
