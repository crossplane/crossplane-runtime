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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
)

// A LocalConnectionSecretOwner may create and manage a connection secret in its
// own namespace.
type LocalConnectionSecretOwner interface {
	metav1.Object
	LocalConnectionSecretWriterTo
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
// another Bindable, such as a resource claim or managed resource.
func IsBindable(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseUnbound
}

// IsBound returns true if the supplied Bindable is bound to another Bindable,
// such as a resource claim or managed resource.
func IsBound(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseBound
}

// IsConditionTrue returns if condition status is true
func IsConditionTrue(c v1alpha1.Condition) bool {
	return c.Status == corev1.ConditionTrue
}
