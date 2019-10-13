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
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
)

// A ConfiguratorChain chains multiple configurators.
type ConfiguratorChain []ManagedConfigurator

// Configure calls each ManagedConfigurator serially. It returns the first
// error it encounters, if any.
func (cc ConfiguratorChain) Configure(ctx context.Context, cm Claim, cs Class, mg Managed) error {
	for _, c := range cc {
		if err := c.Configure(ctx, cm, cs, mg); err != nil {
			return err
		}
	}
	return nil
}

// An ObjectMetaConfigurator sets standard object metadata for a dynamically
// provisioned resource, deriving it from a class and claim.
type ObjectMetaConfigurator struct {
	typer runtime.ObjectTyper
}

// NewObjectMetaConfigurator returns a new ObjectMetaConfigurator.
func NewObjectMetaConfigurator(t runtime.ObjectTyper) *ObjectMetaConfigurator {
	return &ObjectMetaConfigurator{typer: t}
}

// Configure the supplied Managed resource's object metadata.
func (c *ObjectMetaConfigurator) Configure(_ context.Context, cm Claim, _ Class, mg Managed) error {
	mg.SetGenerateName(fmt.Sprintf("%s-%s-", cm.GetNamespace(), cm.GetName()))
	if meta.GetExternalName(cm) != "" {
		meta.SetExternalName(mg, meta.GetExternalName(cm))
	}

	// TODO(negz): Avoid setting this owner reference? Kubernetes specifies that
	// cluster scoped resources cannot have namespaced owners, by design, but
	// the owner reference appears to work for cascading deletes.
	// https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents

	// TODO(negz): We probably want to use the resource's reclaim policy, not
	// Kubernetes garbage collection, to determine whether to delete a managed
	// resource when its claim is deleted per
	// https://github.com/crossplaneio/crossplane/issues/550
	mg.SetOwnerReferences([]v1.OwnerReference{meta.AsOwner(meta.ReferenceTo(cm, MustGetKind(cm, c.typer)))})

	return nil
}
