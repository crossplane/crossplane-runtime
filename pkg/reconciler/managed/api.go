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

package managed

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateOrUpdateSecret = "cannot create or update connection secret"
	errGetSecret            = "cannot get connection secret"
	errUpdateManaged        = "cannot update managed resource"
	errUpdateManagedStatus  = "cannot update managed resource status"
	errResolveReferences    = "cannot resolve references"
)

// NameAsExternalName writes the name of the managed resource to
// the external name annotation field in order to be used as name of
// the external resource in provider.
type NameAsExternalName struct{ client client.Client }

// NewNameAsExternalName returns a new NameAsExternalName.
func NewNameAsExternalName(c client.Client) *NameAsExternalName {
	return &NameAsExternalName{client: c}
}

// Initialize the given managed resource.
func (a *NameAsExternalName) Initialize(ctx context.Context, mg resource.Managed) error {
	if meta.GetExternalName(mg) != "" {
		return nil
	}
	meta.SetExternalName(mg, mg.GetName())
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}

// DefaultProviderConfig fills the ProviderConfigRef with `default` if it's left
// empty.
type DefaultProviderConfig struct{ client client.Client }

// NewDefaultProviderConfig returns a new DefaultProviderConfig.
func NewDefaultProviderConfig(c client.Client) *DefaultProviderConfig {
	return &DefaultProviderConfig{client: c}
}

// Initialize the given managed resource.
func (a *DefaultProviderConfig) Initialize(ctx context.Context, mg resource.Managed) error {
	if mg.GetProviderConfigReference() != nil {
		return nil
	}
	mg.SetProviderConfigReference(&v1alpha1.Reference{Name: "default"})
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}

// An APISecretPublisher publishes ConnectionDetails by submitting a Secret to a
// Kubernetes API server.
type APISecretPublisher struct {
	secret resource.ClientApplicator
	typer  runtime.ObjectTyper
}

// NewAPISecretPublisher returns a new APISecretPublisher.
func NewAPISecretPublisher(c client.Client, ot runtime.ObjectTyper) *APISecretPublisher {
	// NOTE(negz): We transparently inject a ClientApplicator in order to
	// maintain backward compatibility with the original API of this function.
	return &APISecretPublisher{
		secret: resource.ClientApplicator{
			Applicator: resource.NewAPIPatchingApplicator(c),
			Client:     c,
		},
		typer: ot,
	}
}

// PublishConnection publishes the supplied ConnectionDetails to a Secret in the
// same namespace as the supplied Managed resource. It is a no-op if the secret
// already exists with the supplied ConnectionDetails.
func (a *APISecretPublisher) PublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
	// This resource does not want to expose a connection secret.
	if mg.GetWriteConnectionSecretToReference() == nil {
		return nil
	}

	s := resource.ConnectionSecretFor(mg, resource.MustGetKind(mg, a.typer))
	s.Data = c
	return errors.Wrap(a.secret.Apply(ctx, s, resource.ConnectionSecretMustBeControllableBy(mg.GetUID())), errCreateOrUpdateSecret)
}

// UnpublishConnection defers deletion to Kubernetes garbage collection, but
// returns an error if the secret has any owners beside the managed resource
// that controls it.
func (a *APISecretPublisher) UnpublishConnection(ctx context.Context, mg resource.Managed, _ ConnectionDetails) error {
	if mg.GetWriteConnectionSecretToReference() == nil {
		return nil
	}
	s := resource.ConnectionSecretFor(mg, resource.MustGetKind(mg, a.typer))
	nn := types.NamespacedName{Namespace: s.GetNamespace(), Name: s.GetName()}
	if err := a.secret.Get(ctx, nn, s); resource.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, errGetSecret)
	}

	for _, ref := range s.GetOwnerReferences() {
		if ref.UID != mg.GetUID() {
			return errors.Errorf("refusing to unpublish connection secret owned by %s %q", ref.Kind, ref.Name)
		}
	}

	// We don't need to actually delete the connection secret; Kubernetes
	// garbage collection will take care of that.
	return nil
}

// An APISimpleReferenceResolver resolves references from one managed resource
// to others by calling the referencing resource's ResolveReferences method, if
// any.
type APISimpleReferenceResolver struct {
	client client.Client
}

// NewAPISimpleReferenceResolver returns a ReferenceResolver that resolves
// references from one managed resource to others by calling the referencing
// resource's ResolveReferences method, if any.
func NewAPISimpleReferenceResolver(c client.Client) *APISimpleReferenceResolver {
	return &APISimpleReferenceResolver{client: c}
}

// ResolveReferences of the supplied managed resource by calling its
// ResolveReferences method, if any.
func (a *APISimpleReferenceResolver) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	rr, ok := mg.(interface {
		ResolveReferences(context.Context, client.Reader) error
	})
	if !ok {
		// This managed resource doesn't have any references to resolve.
		return nil
	}

	existing := mg.DeepCopyObject()
	if err := rr.ResolveReferences(ctx, a.client); err != nil {
		return errors.Wrap(err, errResolveReferences)
	}

	if cmp.Equal(existing, mg) {
		// The resource didn't change during reference resolution.
		return nil
	}

	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}
