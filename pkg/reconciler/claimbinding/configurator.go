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

package claimbinding

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
)

// A ConfiguratorChain chains multiple configurators.
type ConfiguratorChain []ManagedConfigurator

// Configure calls each ManagedConfigurator serially. It returns the first
// error it encounters, if any.
func (cc ConfiguratorChain) Configure(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	for _, c := range cc {
		if err := c.Configure(ctx, cm, cs, mg); err != nil {
			return err
		}
	}
	return nil
}

// An ObjectMetaConfigurator sets standard object metadata for a dynamically
// provisioned resource, deriving it from a class and claim. It is deprecated;
// use ConfigureNames instead.
type ObjectMetaConfigurator struct{}

// NewObjectMetaConfigurator returns a new ObjectMetaConfigurator.
func NewObjectMetaConfigurator(_ runtime.ObjectTyper) *ObjectMetaConfigurator {
	return &ObjectMetaConfigurator{}
}

// Configure the supplied Managed resource's object metadata.
func (c *ObjectMetaConfigurator) Configure(ctx context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	return ConfigureNames(ctx, cm, cs, mg)
}

// ConfigureNames configures the name and external name of the supplied managed
// resource. The managed resource name is derived from the supplied resource
// claim, in the form {claim-namespace}-{claim-name}-{random-string}. The
// resource claim's external name annotation, if any, is propagated to the
// managed resource.
func ConfigureNames(_ context.Context, cm resource.Claim, _ resource.Class, mg resource.Managed) error {
	mg.SetGenerateName(fmt.Sprintf("%s-%s-", cm.GetNamespace(), cm.GetName()))
	if meta.GetExternalName(cm) != "" {
		meta.SetExternalName(mg, meta.GetExternalName(cm))
	}

	return nil
}

// ConfigureReclaimPolicy configures the reclaim policy of the supplied managed
// resource. If the managed resource _already has_ a reclaim policy (for example
// because one was set by another configurator) it is respected. Otherwise the
// reclaim policy is copied from the resource class. If the resource class does
// not specify a reclaim policy, the managed resource's policy is set to
// "Delete".
func ConfigureReclaimPolicy(_ context.Context, _ resource.Claim, cs resource.Class, mg resource.Managed) error {
	if mg.GetReclaimPolicy() != "" {
		return nil
	}

	mg.SetReclaimPolicy(cs.GetReclaimPolicy())

	if mg.GetReclaimPolicy() == "" {
		mg.SetReclaimPolicy(v1alpha1.ReclaimDelete)
	}

	return nil
}
