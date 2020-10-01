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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	o, ok := obj.(metav1.Object)
	if !ok {
		return
	}

	// Otherwise we should enqueue a request for the objects it propagates to.
	for nn := range meta.AllowsPropagationTo(o) {
		queue.Add(reconcile.Request{NamespacedName: nn})
	}
}

// EnqueueRequestForProviderConfig enqueues a reconcile.Request for a referenced
// ProviderConfig.
type EnqueueRequestForProviderConfig struct{}

// Create adds a NamespacedName for the supplied CreateEvent if its Object is a
// ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	addProviderConfig(evt.Object, q)
}

// Update adds a NamespacedName for the supplied UpdateEvent if its Objects are
// a ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	addProviderConfig(evt.ObjectOld, q)
	addProviderConfig(evt.ObjectNew, q)
}

// Delete adds a NamespacedName for the supplied DeleteEvent if its Object is a
// ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	addProviderConfig(evt.Object, q)
}

// Generic adds a NamespacedName for the supplied GenericEvent if its Object is
// a ProviderConfigReferencer.
func (e *EnqueueRequestForProviderConfig) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	addProviderConfig(evt.Object, q)
}

func addProviderConfig(obj runtime.Object, queue adder) {
	pcr, ok := obj.(RequiredProviderConfigReferencer)
	if !ok {
		return
	}

	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: pcr.GetProviderConfigReference().Name}})
}
