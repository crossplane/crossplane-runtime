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

package controller

import (
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type EngineOption func(*Engine)

func WithLogger(l logging.Logger) EngineOption {
	return func(e *Engine) {
		e.log = l
	}
}

func NewEngine(mgr manager.Manager, o ...EngineOption) *Engine {
	e := &Engine{
		mgr:     mgr,
		started: map[string]chan struct{}{},

		log: logging.NewNopLogger(),
	}

	for _, eo := range o {
		eo(e)
	}

	return e
}

type Engine struct {
	mgr     manager.Manager
	started map[string]chan struct{}

	log logging.Logger
}

func (e *Engine) IsRunning(name string) bool {
	stop, started := e.started[name]
	if !started {
		return false
	}

	// Nothing ever writes to the stop channel. If we can read from it it's
	// closed and therefore the controller is no longer running. If we can't
	// immediately read from it, the channel is not closed, and therefore the
	// controller is still running.
	select {
	case <-stop:
		return false
	default:
		return true
	}
}

func (e *Engine) Stop(name string) {
	stop, exists := e.started[name]
	if !exists {
		return
	}
	close(stop)
	delete(e.started, name)
}

// TODO(negz): Accept alternative EventHandlers
// TODO(negz): Accept a different map of object kind to EventHandler?

// Source, eventhandler, predicates.
// Source is fixed-ish. Different object, same cache.

type For struct {
	Kind       runtime.Object
	Handler    handler.EventHandler
	Predicates []predicate.Predicate
}

// TODO(negz): Should we try to detect and restart crashed controllers? What
// would cause a controller to crash? Maybe we should crash the whole process,
// which is presumably what controller-runtime does for a normal controller.
func (e *Engine) Start(name string, r reconcile.Reconciler, f ...For) error {
	if e.IsRunning(name) {
		return nil
	}

	stop := make(chan struct{})
	e.started[name] = stop

	// Each controller gets its own cache because there's currently no way to
	// stop an informer. In practice a controller-runtime cache is a map of
	// kinds to informers. If we delete the CRD for a kind we need to stop the
	// relevant informer, or it will spew errors about the kind not existing.
	ca, err := cache.New(e.mgr.GetConfig(), cache.Options{
		Scheme: e.mgr.GetScheme(),
		Mapper: e.mgr.GetRESTMapper(),
		// TODO(negz): Add a WithResyncInterval option?
	})
	if err != nil {
		return err
	}

	go func() {
		<-e.mgr.Leading()
		if err := ca.Start(stop); err != nil {
			e.log.Debug("cannot start controller cache", "controller", name, "error", err)
		}
	}()

	ctrl, err := controller.NewUnmanaged(name, e.mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create an unmanaged controller")
	}

	for _, fr := range f {
		if err := ctrl.Watch(source.NewKindWithCache(fr.Kind, ca), fr.Handler, fr.Predicates...); err != nil {
			return errors.Wrapf(err, "cannot watch %T", fr.Kind)
		}
	}

	// TODO(negz): Make these logs look like upstream controller-runtime start
	// and stop log lines?
	go func() {
		<-e.mgr.Leading()
		e.log.Debug("Starting", "controller", name)
		if err := ctrl.Start(stop); err != nil {
			e.log.Debug("Cannot start controller", "controller", name, "error", err)
			return
		}
		e.log.Debug("Stopped", "controller", name)
	}()

	return nil
}
