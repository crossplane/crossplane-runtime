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
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

type adder interface {
	Add(item interface{})
}

// EnqueueRequestForClaim enqueues a reconcile.Request for the NamespacedName
// of a ClaimReferencer's ClaimReference.
type EnqueueRequestForClaim struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// ClaimReferencers.
func (e *EnqueueRequestForClaim) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.ObjectOld, q)
	addClaim(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is a
// ClaimReferencer.
func (e *EnqueueRequestForClaim) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addClaim(evt.Object, q)
}

func addClaim(obj runtime.Object, queue adder) {
	if cr, ok := obj.(ClaimReferencer); ok && cr.GetClaimReference() != nil {
		queue.Add(reconcile.Request{NamespacedName: meta.NamespacedNameOf(cr.GetClaimReference())})
	}
}

// EnqueueRequestForPropagated enqueues a reconcile.Request for the
// NamespacedName of a propagated object, i.e. an object with propagation
// metadata annotations.
type EnqueueRequestForPropagated struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is
// propagated.
func (e *EnqueueRequestForPropagated) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addPropagated(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// propagated.
func (e *EnqueueRequestForPropagated) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addPropagated(evt.ObjectOld, q)
	addPropagated(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is
// propagated.
func (e *EnqueueRequestForPropagated) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addPropagated(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// propagated.
func (e *EnqueueRequestForPropagated) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addPropagated(evt.Object, q)
}

func addPropagated(obj runtime.Object, queue adder) {
	ao, ok := obj.(interface {
		GetAnnotations() map[string]string
	})
	if !ok {
		return
	}

	a := ao.GetAnnotations()

	for key, val := range a {
		if !strings.HasPrefix(key, AnnotationKeyPropagateToPrefix) {
			continue
		}
		t := strings.Split(val, AnnotationDelimiter)
		if len(t) != 2 {
			continue
		}
		switch {
		case t[0] == "":
			continue
		case t[1] == "":
			continue
		default:
			queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: t[0],
				Name:      t[1],
			}})
		}
	}
}
