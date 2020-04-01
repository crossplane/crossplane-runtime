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
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Error strings.
const (
	errGetSecret            = "cannot get managed resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errUpdateSecret         = "cannot update connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
)

// An APIManagedConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIManagedConnectionPropagator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIManagedConnectionPropagator returns a new APIManagedConnectionPropagator.
func NewAPIManagedConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIManagedConnectionPropagator {
	return &APIManagedConnectionPropagator{client: c, typer: t}
}

// PropagateConnection details from the supplied resource to the supplied claim.
func (a *APIManagedConnectionPropagator) PropagateConnection(ctx context.Context, o LocalConnectionSecretOwner, mg Managed) error {
	// Either this resource does not expose a connection secret, or this claim
	// does not want one.
	if mg.GetWriteConnectionSecretToReference() == nil || o.GetWriteConnectionSecretToReference() == nil {
		return nil
	}

	n := types.NamespacedName{
		Namespace: mg.GetWriteConnectionSecretToReference().Namespace,
		Name:      mg.GetWriteConnectionSecretToReference().Name,
	}
	mgcs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, mgcs); err != nil {
		return errors.Wrap(err, errGetSecret)
	}

	// Make sure the managed resource is the controller of the connection secret
	// it references before we propagate it. This ensures a managed resource
	// cannot use Crossplane to circumvent RBAC by propagating a secret it does
	// not own.
	if c := metav1.GetControllerOf(mgcs); c == nil || c.UID != mg.GetUID() {
		return errors.New(errSecretConflict)
	}

	cmcs := LocalConnectionSecretFor(o, MustGetKind(o, a.typer))
	if _, err := util.CreateOrUpdate(ctx, a.client, cmcs, func() error {
		// Inside this anonymous function cmcs could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.
		if c := metav1.GetControllerOf(cmcs); c == nil || c.UID != o.GetUID() {
			return errors.New(errSecretConflict)
		}
		cmcs.Data = mgcs.Data
		meta.AddAnnotations(cmcs, map[string]string{
			AnnotationKeyPropagateFromNamespace: mgcs.GetNamespace(),
			AnnotationKeyPropagateFromName:      mgcs.GetName(),
			AnnotationKeyPropagateFromUID:       string(mgcs.GetUID()),
		})
		return nil
	}); err != nil {
		return errors.Wrap(err, errCreateOrUpdateSecret)
	}

	k := strings.Join([]string{AnnotationKeyPropagateToPrefix, string(cmcs.GetUID())}, AnnotationDelimiter)
	v := strings.Join([]string{cmcs.GetNamespace(), cmcs.GetName()}, AnnotationDelimiter)
	meta.AddAnnotations(mgcs, map[string]string{k: v})

	return errors.Wrap(a.client.Update(ctx, mgcs), errUpdateSecret)
}

// An APIApplicator applies changes to an object by either creating or patching
// it in a Kubernetes API server.
type APIApplicator struct {
	client client.Client
}

// NewAPIApplicator returns an Applicator that applies changes to the Kubernetes
// API server.
func NewAPIApplicator(c client.Client) *APIApplicator {
	return &APIApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does.
func (a *APIApplicator) Apply(ctx context.Context, o runtime.Object, ao ...ApplyOption) error {
	m, ok := o.(metav1.Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	desired := o.DeepCopyObject()

	err := a.client.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, o)
	if kerrors.IsNotFound(err) {
		// TODO(negz): Apply ApplyOptions here too?
		return errors.Wrap(a.client.Create(ctx, o), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	for _, fn := range ao {
		if err := fn(ctx, o, desired); err != nil {
			return err
		}
	}

	return errors.Wrap(a.client.Patch(ctx, o, &patch{desired}), "cannot patch object")
}

type patch struct{ from runtime.Object }

func (p *patch) Type() types.PatchType                 { return types.MergePatchType }
func (p *patch) Data(_ runtime.Object) ([]byte, error) { return json.Marshal(p.from) }
