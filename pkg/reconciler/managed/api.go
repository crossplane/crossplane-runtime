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
	"net/url"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateOrUpdateSecret = "cannot create or update connection secret"
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

// ClusterDestructor cleans object out of a cluster before deletion such that
// any external infrastructure that was provisioned on their behalf, such as a
// load balancer or firewall rule, are cleaned up before the cluster is deleted.
type ClusterDestructor struct{ client client.Client }

// NewClusterDestructor returns a new ClusterDestructor.
func NewClusterDestructor(c client.Client) *ClusterDestructor {
	return &ClusterDestructor{client: c}
}

// Destruct the given managed resource.
func (a *ClusterDestructor) Destruct(ctx context.Context, mg resource.Managed) error {
	ref := mg.GetWriteConnectionSecretToReference()
	s := &corev1.Secret{}
	if err := a.client.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, s); err != nil {
		return err
	}

	config := &rest.Config{}
	if len(s.Data[v1alpha1.ResourceCredentialsSecretKubeconfigKey]) != 0 {
		conf, err := clientcmd.RESTConfigFromKubeConfig(s.Data[v1alpha1.ResourceCredentialsSecretKubeconfigKey])
		if err != nil {
			return err
		}
		config = conf
	} else {
		u, err := url.Parse(string(s.Data[v1alpha1.ResourceCredentialsSecretEndpointKey]))
		if err != nil {
			return errors.Wrap(err, "cannot parse Kubernetes endpoint as URL")
		}

		config = &rest.Config{
			Host:     u.String(),
			Username: string(s.Data[v1alpha1.ResourceCredentialsSecretUserKey]),
			Password: string(s.Data[v1alpha1.ResourceCredentialsSecretPasswordKey]),
			TLSClientConfig: rest.TLSClientConfig{
				// This field's godoc claims clients will use 'the hostname used to
				// contact the server' when it is left unset. In practice clients
				// appear to use the URL, including scheme and port.
				ServerName: u.Hostname(),
				CAData:     s.Data[v1alpha1.ResourceCredentialsSecretCAKey],
				CertData:   s.Data[v1alpha1.ResourceCredentialsSecretClientCertKey],
				KeyData:    s.Data[v1alpha1.ResourceCredentialsSecretClientKeyKey],
			},
			BearerToken: string(s.Data[v1alpha1.ResourceCredentialsSecretTokenKey]),
		}
	}

	kc, err := client.New(config, client.Options{})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes client")
	}

	deletionPolicy := metav1.DeletePropagationForeground
	if err := kc.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "wordpress"}}, &client.DeleteOptions{PropagationPolicy: &deletionPolicy}); err != nil {
		return err
	}

	return nil
}

// An APISecretPublisher publishes ConnectionDetails by submitting a Secret to a
// Kubernetes API server.
type APISecretPublisher struct {
	secret resource.Applicator
	typer  runtime.ObjectTyper
}

// NewAPISecretPublisher returns a new APISecretPublisher.
func NewAPISecretPublisher(c client.Client, ot runtime.ObjectTyper) *APISecretPublisher {
	// NOTE(negz): We transparently inject an APIPatchingApplicator in order to maintain
	// backward compatibility with the original API of this function.
	return &APISecretPublisher{secret: resource.NewAPIPatchingApplicator(c), typer: ot}
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

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APISecretPublisher) UnpublishConnection(ctx context.Context, mg resource.Managed, c ConnectionDetails) error {
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
