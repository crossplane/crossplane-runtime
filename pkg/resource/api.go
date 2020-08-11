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
	"encoding/json"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Error strings.
const (
	errGetSecret            = "cannot get managed resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errUpdateSecret         = "cannot update connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"

	errUpdateObject = "cannot update object"
)

// An APIManagedConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIManagedConnectionPropagator struct {
	Propagator ConnectionPropagator
}

// NewAPIManagedConnectionPropagator returns a new APIConnectionPropagator.
//
// Deprecated: Use NewAPIConnectionPropagator.
func NewAPIManagedConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIManagedConnectionPropagator {
	return &APIManagedConnectionPropagator{Propagator: NewAPIConnectionPropagator(c, t)}
}

// PropagateConnection details from the supplied resource.
func (a *APIManagedConnectionPropagator) PropagateConnection(ctx context.Context, to LocalConnectionSecretOwner, mg Managed) error {
	return a.Propagator.PropagateConnection(ctx, to, mg)
}

// An APIConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIConnectionPropagator struct {
	client ClientApplicator
	typer  runtime.ObjectTyper
}

// NewAPIConnectionPropagator returns a new APIConnectionPropagator.
func NewAPIConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIConnectionPropagator {
	return &APIConnectionPropagator{
		client: ClientApplicator{Client: c, Applicator: NewAPIUpdatingApplicator(c)},
		typer:  t,
	}
}

// PropagateConnection details from the supplied resource.
func (a *APIConnectionPropagator) PropagateConnection(ctx context.Context, to LocalConnectionSecretOwner, from ConnectionSecretOwner) error {
	// Either from does not expose a connection secret, or to does not want one.
	if from.GetWriteConnectionSecretToReference() == nil || to.GetWriteConnectionSecretToReference() == nil {
		return nil
	}

	n := types.NamespacedName{
		Namespace: from.GetWriteConnectionSecretToReference().Namespace,
		Name:      from.GetWriteConnectionSecretToReference().Name,
	}
	fs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, fs); err != nil {
		return errors.Wrap(err, errGetSecret)
	}

	// Make sure the managed resource is the controller of the connection secret
	// it references before we propagate it. This ensures a managed resource
	// cannot use Crossplane to circumvent RBAC by propagating a secret it does
	// not own.
	if c := metav1.GetControllerOf(fs); c == nil || c.UID != from.GetUID() {
		return errors.New(errSecretConflict)
	}

	ts := LocalConnectionSecretFor(to, MustGetKind(to, a.typer))
	ts.Data = fs.Data

	meta.AllowPropagation(fs, ts)

	if err := a.client.Apply(ctx, ts, ConnectionSecretMustBeControllableBy(to.GetUID())); err != nil {
		return errors.Wrap(err, errCreateOrUpdateSecret)
	}

	return errors.Wrap(a.client.Update(ctx, fs), errUpdateSecret)
}

// An APIPatchingApplicator applies changes to an object by either creating or
// patching it in a Kubernetes API server.
type APIPatchingApplicator struct {
	client client.Client
}

// NewAPIPatchingApplicator returns an Applicator that applies changes to an
// object by either creating or patching it in a Kubernetes API server.
func NewAPIPatchingApplicator(c client.Client) *APIPatchingApplicator {
	return &APIPatchingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does. If the object does it exist it will always
// be patched, regardless of resource version.
func (a *APIPatchingApplicator) Apply(ctx context.Context, o runtime.Object, ao ...ApplyOption) error { //nolint:gocyclo
	m, ok := o.(metav1.Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	empty := NewEmptyInstance(o)
	desired := o.DeepCopyObject()

	if m.GetName() == "" && m.GetGenerateName() != "" {
		// This for loop is written twice but run only once for an object. If
		// we put it out of these if blocks, then they will be called all the time
		// in case object exists unnecessarily and also with a wrong input, i.e.
		// empty object is a wrong input if the object exists.
		for _, fn := range ao {
			if err := fn(ctx, empty, desired); err != nil {
				return err
			}
		}
		return errors.Wrap(a.client.Create(ctx, desired), "cannot create object")
	}
	// In the end of the function, "o" needs to be returned as the most up-to-date
	// form of the object that exists in the api-server. Hence, we stop using
	// "empty" object the moment we realize the object already exists.
	err := a.client.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, o)
	if kerrors.IsNotFound(err) {
		for _, fn := range ao {
			if err := fn(ctx, empty, desired); err != nil {
				return err
			}
		}
		return errors.Wrap(a.client.Create(ctx, desired), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	for _, fn := range ao {
		if err := fn(ctx, o, desired); err != nil {
			return err
		}
	}

	// TODO(negz): Allow callers to override the kind of patch used.
	return errors.Wrap(a.client.Patch(ctx, o, &patch{desired}), "cannot patch object")
}

type patch struct{ from runtime.Object }

func (p *patch) Type() types.PatchType                 { return types.MergePatchType }
func (p *patch) Data(_ runtime.Object) ([]byte, error) { return json.Marshal(p.from) }

// An APIUpdatingApplicator applies changes to an object by either creating or
// updating it in a Kubernetes API server.
type APIUpdatingApplicator struct {
	client client.Client
}

// NewAPIUpdatingApplicator returns an Applicator that applies changes to an
// object by either creating or updating it in a Kubernetes API server.
func NewAPIUpdatingApplicator(c client.Client) *APIUpdatingApplicator {
	return &APIUpdatingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or updated if it does. If the object does exist and no
// ApplyOptions are passed, the update will be a no-op.
func (a *APIUpdatingApplicator) Apply(ctx context.Context, o runtime.Object, ao ...ApplyOption) error { //nolint:gocyclo
	m, ok := o.(metav1.Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	empty := NewEmptyInstance(o)
	desired := o.DeepCopyObject()

	if m.GetName() == "" && m.GetGenerateName() != "" {
		// This for loop is written twice but run only once for an object. If
		// we put it out of these if blocks, then they will be called all the time
		// in case object exists unnecessarily and also with a wrong input, i.e.
		// empty object is a wrong input if the object exists.
		for _, fn := range ao {
			if err := fn(ctx, empty, desired); err != nil {
				return err
			}
		}
		return errors.Wrap(a.client.Create(ctx, desired), "cannot create object")
	}
	// In the end of the function, "o" needs to be returned as the most up-to-date
	// form of the object that exists in the api-server. Hence, we stop using
	// "empty" object the moment we realize the object already exists.
	err := a.client.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, o)
	if kerrors.IsNotFound(err) {
		for _, fn := range ao {
			if err := fn(ctx, empty, desired); err != nil {
				return err
			}
		}
		return errors.Wrap(a.client.Create(ctx, desired), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	for _, fn := range ao {
		if err := fn(ctx, o, desired); err != nil {
			return err
		}
	}

	return errors.Wrap(a.client.Update(ctx, o), "cannot update object")
}

// An APIFinalizer adds and removes finalizers to and from a resource.
type APIFinalizer struct {
	client    client.Client
	finalizer string
}

// NewAPIFinalizer returns a new APIFinalizer.
func NewAPIFinalizer(c client.Client, finalizer string) *APIFinalizer {
	return &APIFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Managed resource.
func (a *APIFinalizer) AddFinalizer(ctx context.Context, obj Object) error {
	if meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(obj, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, obj), errUpdateObject)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIFinalizer) RemoveFinalizer(ctx context.Context, obj Object) error {
	if !meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.RemoveFinalizer(obj, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, obj)), errUpdateObject)
}

// A FinalizerFns satisfy the Finalizer interface.
type FinalizerFns struct {
	AddFinalizerFn    func(ctx context.Context, obj Object) error
	RemoveFinalizerFn func(ctx context.Context, obj Object) error
}

// AddFinalizer to the supplied resource.
func (f FinalizerFns) AddFinalizer(ctx context.Context, obj Object) error {
	return f.AddFinalizerFn(ctx, obj)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) RemoveFinalizer(ctx context.Context, obj Object) error {
	return f.RemoveFinalizerFn(ctx, obj)
}
