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

// SecretTypeConnection is the type of Crossplane connection secrets.
const SecretTypeConnection corev1.SecretType = "connection.crossplane.io/v1alpha1"

// External resources are tagged/labelled with the following keys in the cloud
// provider API if the type supports.
const (
	ExternalResourceTagKeyKind     = "crossplane-kind"
	ExternalResourceTagKeyName     = "crossplane-name"
	ExternalResourceTagKeyClass    = "crossplane-class"
	ExternalResourceTagKeyProvider = "crossplane-provider"
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

// A ManagedKind contains the type metadata for a kind of managed.
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
		Type: SecretTypeConnection,
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
		Type: SecretTypeConnection,
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

// An Applicator applies changes to an object.
type Applicator interface {
	Apply(context.Context, runtime.Object, ...ApplyOption) error
}

// A ClientApplicator may be used to build a single 'client' that satisfies both
// client.Client and Applicator.
type ClientApplicator struct {
	client.Client
	Applicator
}

// An ApplyFn is a function that satisfies the Applicator interface.
type ApplyFn func(context.Context, runtime.Object, ...ApplyOption) error

// Apply changes to the supplied object.
func (fn ApplyFn) Apply(ctx context.Context, o runtime.Object, ao ...ApplyOption) error {
	return fn(ctx, o, ao...)
}

// An ApplyOption is called before patching the current object to match the
// desired object. ApplyOptions are not called if no current object exists.
type ApplyOption func(ctx context.Context, current, desired runtime.Object) error

// MustBeControllableBy requires that the current object is controllable by an
// object with the supplied UID. An object is controllable if its controller
// reference matches the supplied UID, or it has no controller reference.
func MustBeControllableBy(u types.UID) ApplyOption {
	return func(_ context.Context, current, _ runtime.Object) error {
		c := metav1.GetControllerOf(current.(metav1.Object))
		if c == nil {
			return nil
		}

		if c.UID != u {
			return errors.Errorf("existing object is not controlled by UID %q", u)

		}
		return nil
	}
}

// ConnectionSecretMustBeControllableBy requires that the current object is a
// connection secret that is controllable by an object with the supplied UID.
// Contemporary connection secrets are of SecretTypeConnection, while legacy
// connection secrets are of corev1.SecretTypeOpaque. Contemporary connection
// secrets are considered controllable if they are already controlled by the
// supplied UID, or have no controller reference. Legacy connection secrets are
// only considered controllable if they are already controlled by the supplied
// UID. It is not safe to assume legacy connection secrets without a controller
// reference are controllable because they are indistinguishable from Kubernetes
// secrets that have nothing to do with Crossplane.
func ConnectionSecretMustBeControllableBy(u types.UID) ApplyOption {
	return func(_ context.Context, current, _ runtime.Object) error {
		s := current.(*corev1.Secret)
		c := metav1.GetControllerOf(s)

		switch {
		case c == nil && s.Type != SecretTypeConnection:
			return errors.Errorf("refusing to modify uncontrolled secret of type %q", s.Type)
		case c == nil:
			return nil
		case c.UID != u:
			return errors.Errorf("existing secret is not controlled by UID %q", u)
		}

		return nil
	}
}

// ControllersMustMatch requires the current object to have a controller
// reference, and for that controller reference to match the controller
// reference of the desired object.
//
// Deprecated: Use ControllableBy.
func ControllersMustMatch() ApplyOption {
	return func(_ context.Context, current, desired runtime.Object) error {
		if !meta.HaveSameController(current.(metav1.Object), desired.(metav1.Object)) {
			return errors.New("existing object has a different (or no) controller")
		}
		return nil
	}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does.
//
// Deprecated: use APIPatchingApplicator instead.
func Apply(ctx context.Context, c client.Client, o runtime.Object, ao ...ApplyOption) error {
	return NewAPIPatchingApplicator(c).Apply(ctx, o, ao...)
}

// GetExternalTags returns the identifying tags to be used to tag the external
// resource in provider API.
func GetExternalTags(mg Managed) map[string]string {
	tags := map[string]string{
		ExternalResourceTagKeyKind: strings.ToLower(mg.GetObjectKind().GroupVersionKind().GroupKind().String()),
		ExternalResourceTagKeyName: mg.GetName(),
	}
	if mg.GetClassReference() != nil {
		tags[ExternalResourceTagKeyClass] = mg.GetClassReference().Name
	}
	if mg.GetProviderReference() != nil {
		tags[ExternalResourceTagKeyProvider] = mg.GetProviderReference().Name
	}
	return tags
}
