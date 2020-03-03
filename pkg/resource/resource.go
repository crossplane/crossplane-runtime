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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Supported resources with all of these annotations will be fully or partially
// propagated to the named resource of the same kind, assuming it exists and
// consents to propagation.
const (
	AnnotationKeyPropagateToPrefix = "to.propagate.crossplane.io"

	AnnotationKeyPropagateFromNamespace = "from.propagate.crossplane.io/namespace"
	AnnotationKeyPropagateFromName      = "from.propagate.crossplane.io/name"
	AnnotationKeyPropagateFromUID       = "from.propagate.crossplane.io/uid"

	AnnotationDelimiter = "/"
)

// A ClaimKind contains the type metadata for a kind of resource claim.
type ClaimKind schema.GroupVersionKind

// A ClassKind contains the type metadata for a kind of resource class.
type ClassKind schema.GroupVersionKind

// List returns the list kind associated with a ClassKind.
func (k ClassKind) List() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   k.Group,
		Version: k.Version,
		Kind:    k.Kind + "List",
	}
}

// A ManagedKind contains the type metadata for a kind of managed
type ManagedKind schema.GroupVersionKind

// A TargetKind contains the type metadata for a kind of target resource.
type TargetKind schema.GroupVersionKind

// A LocalConnectionSecretOwner may create and manage a connection secret in its
// own namespace.
type LocalConnectionSecretOwner interface {
	runtime.Object
	metav1.Object

	LocalConnectionSecretWriterTo
}

// A ManagedConnectionPropagator is responsible for propagating information
// required to connect to a managed resource (for example the connection secret)
// from the managed resource to its resource claim.
type ManagedConnectionPropagator interface {
	PropagateConnection(ctx context.Context, o LocalConnectionSecretOwner, mg Managed) error
}

// A ManagedConnectionPropagatorFn is a function that satisfies the
// ManagedConnectionPropagator interface.
type ManagedConnectionPropagatorFn func(ctx context.Context, o LocalConnectionSecretOwner, mg Managed) error

// PropagateConnection information from the supplied managed resource to the
// supplied resource claim.
func (fn ManagedConnectionPropagatorFn) PropagateConnection(ctx context.Context, o LocalConnectionSecretOwner, mg Managed) error {
	return fn(ctx, o, mg)
}

// LocalConnectionSecretFor creates a connection secret in the namespace of the
// supplied LocalConnectionSecretOwner, assumed to be of the supplied kind.
func LocalConnectionSecretFor(o LocalConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetNamespace(),
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.ReferenceTo(o, kind))},
		},
		Data: make(map[string][]byte),
	}
}

// A ConnectionSecretOwner may create and manage a connection secret in an
// arbitrary namespace.
type ConnectionSecretOwner interface {
	runtime.Object
	metav1.Object

	ConnectionSecretWriterTo
}

// ConnectionSecretFor creates a connection for the supplied
// ConnectionSecretOwner, assumed to be of the supplied kind. The secret is
// written to 'default' namespace if the ConnectionSecretOwner does not specify
// a namespace.
func ConnectionSecretFor(o ConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetWriteConnectionSecretToReference().Namespace,
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.ReferenceTo(o, kind))},
		},
		Data: make(map[string][]byte),
	}
}

// MustCreateObject returns a new Object of the supplied kind. It panics if the
// kind is unknown to the supplied ObjectCreator.
func MustCreateObject(kind schema.GroupVersionKind, oc runtime.ObjectCreater) runtime.Object {
	obj, err := oc.New(kind)
	if err != nil {
		panic(err)
	}
	return obj
}

// GetKind returns the GroupVersionKind of the supplied object. It return an
// error if the object is unknown to the supplied ObjectTyper, the object is
// unversioned, or the object does not have exactly one registered kind.
func GetKind(obj runtime.Object, ot runtime.ObjectTyper) (schema.GroupVersionKind, error) {
	kinds, unversioned, err := ot.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, errors.Wrap(err, "cannot get kind of supplied object")
	}
	if unversioned {
		return schema.GroupVersionKind{}, errors.New("supplied object is unversioned")
	}
	if len(kinds) != 1 {
		return schema.GroupVersionKind{}, errors.New("supplied object does not have exactly one kind")
	}
	return kinds[0], nil
}

// MustGetKind returns the GroupVersionKind of the supplied object. It panics if
// the object is unknown to the supplied ObjectTyper, the object is unversioned,
// or the object does not have exactly one registered kind.
func MustGetKind(obj runtime.Object, ot runtime.ObjectTyper) schema.GroupVersionKind {
	gvk, err := GetKind(obj, ot)
	if err != nil {
		panic(err)
	}
	return gvk
}

// An ErrorIs function returns true if an error satisfies a particular condition.
type ErrorIs func(err error) bool

// Ignore any errors that satisfy the supplied ErrorIs function by returning
// nil. Errors that do not satisfy the suppled function are returned unmodified.
func Ignore(is ErrorIs, err error) error {
	if is(err) {
		return nil
	}
	return err
}

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// Kubernetes resource was not found.
func IgnoreNotFound(err error) error {
	return Ignore(kerrors.IsNotFound, err)
}

// ResolveClassClaimValues validates the supplied claim value against the
// supplied resource class value. If both are non-zero they must match.
func ResolveClassClaimValues(classValue, claimValue string) (string, error) {
	if classValue == "" {
		return claimValue, nil
	}
	if claimValue == "" {
		return classValue, nil
	}
	if classValue != claimValue {
		return "", errors.Errorf("claim value [%s] does not match class value [%s]", claimValue, classValue)
	}
	return claimValue, nil
}

// SetBindable indicates that the supplied Bindable is ready for binding to
// another Bindable, such as a resource claim or managed resource by setting its
// binding phase to "Unbound". It is a no-op for Bindables in phases "Bound" or
// "Released", because these phases may not transition back to "Unbound".
func SetBindable(b Bindable) {
	switch b.GetBindingPhase() {
	case v1alpha1.BindingPhaseBound, v1alpha1.BindingPhaseReleased:
		return
	default:
		b.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
	}
}

// IsBindable returns true if the supplied Bindable is ready for binding to
// another Bindable, such as a resource claim or managed
func IsBindable(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseUnbound
}

// IsBound returns true if the supplied Bindable is bound to another Bindable,
// such as a resource claim or managed
func IsBound(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseBound
}

// IsConditionTrue returns if condition status is true
func IsConditionTrue(c v1alpha1.Condition) bool {
	return c.Status == corev1.ConditionTrue
}

// ApplyOptions configure how changes are applied to an object.
type ApplyOptions struct {
	// ControllersMustMatch requires any existing object to have a controller
	// reference, and for that controller reference to match the controller
	// reference of the supplied object.
	ControllersMustMatch bool
}

// An ApplyOption configures how changes are applied to an object.
type ApplyOption func(a *ApplyOptions)

// ControllersMustMatch requires any existing object to have a controller
// reference, and for that controller reference to match the controller
// reference of the supplied object.
func ControllersMustMatch() ApplyOption {
	return func(a *ApplyOptions) {
		a.ControllersMustMatch = true
	}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does.
func Apply(ctx context.Context, c client.Client, o runtime.Object, ao ...ApplyOption) error {
	opts := &ApplyOptions{}
	for _, fn := range ao {
		fn(opts)
	}

	m, ok := o.(metav1.Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	desired := o.DeepCopyObject()

	err := c.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, o)
	if kerrors.IsNotFound(err) {
		return errors.Wrap(c.Create(ctx, o), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	if opts.ControllersMustMatch && !meta.HaveSameController(m, desired.(metav1.Object)) {
		return errors.New("existing object has a different (or no) controller")
	}

	return errors.Wrap(c.Patch(ctx, o, &patch{desired}), "cannot patch object")
}

type patch struct{ from runtime.Object }

func (p *patch) Type() types.PatchType                 { return types.MergePatchType }
func (p *patch) Data(_ runtime.Object) ([]byte, error) { return json.Marshal(p.from) }
