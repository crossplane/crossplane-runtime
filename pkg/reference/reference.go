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

package reference

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errGetManaged  = "cannot get managed resource"
	errListManaged = "cannot get managed resources"
)

// To indicates the kind of managed resource a reference is to.
type To struct {
	Managed resource.Managed
	List    resource.ManagedList
}

// An ExtractValueFn specifies how to extract a value from the resolved managed
// resource.
type ExtractValueFn func(resource.Managed) string

// ExternalName extracts the resolved managed resource's external name from its
// external name annotation.
func ExternalName() ExtractValueFn {
	return func(mg resource.Managed) string {
		return meta.GetExternalName(mg)
	}
}

// A ResolutionRequest requests that a reference to a particular kind of
// managed resource be resolved.
type ResolutionRequest struct {
	CurrentValue string
	Reference    *v1alpha1.Reference
	Selector     *v1alpha1.Selector
	To           To
	Extract      ExtractValueFn
}

// A ResolutionResponse returns the result of a reference resolution. The
// returned values are always safe to set if resolution was successful.
type ResolutionResponse struct {
	ResolvedValue     string
	ResolvedReference *v1alpha1.Reference
}

// A Resolver selects and resolves references to managed resources.
type Resolver interface {
	// Resolve the supplied ResolutionRequest. The returned ResolutionResponse
	// always contains valid values unless an error was returned.
	Resolve(ctx context.Context, req ResolutionRequest) (ResolutionResponse, error)
}

// An APIResolver selects and resolves references to managed resources in the
// Kubernetes API server.
type APIResolver struct {
	client client.Reader
	from   resource.Managed
}

// NewAPIResolver returns a Resolver that selects and resolves references from
// the supplied managed resource to other managed resources in the Kubernetes
// API server.
func NewAPIResolver(c client.Reader, from resource.Managed) *APIResolver {
	return &APIResolver{client: c, from: from}
}

// Resolve the supplied ResolutionRequest. The returned ResolutionResponse
// always contains valid values unless an error was returned.
func (r *APIResolver) Resolve(ctx context.Context, req ResolutionRequest) (ResolutionResponse, error) {
	// Return early if from is being deleted.
	if meta.WasDeleted(r.from) {
		return ResolutionResponse{ResolvedValue: req.CurrentValue, ResolvedReference: req.Reference}, nil
	}

	// Return early if the value was already resolved.
	if req.CurrentValue != "" {
		return ResolutionResponse{ResolvedValue: req.CurrentValue, ResolvedReference: req.Reference}, nil
	}

	// Return early if neither a reference nor a selector exist.
	if req.Reference == nil && req.Selector == nil {
		return ResolutionResponse{ResolvedValue: req.CurrentValue, ResolvedReference: req.Reference}, nil
	}

	// The reference is already set - resolve it.
	if req.Selector == nil {
		to := req.To.Managed
		err := r.client.Get(ctx, types.NamespacedName{Name: req.Reference.Name}, to)
		rsp := ResolutionResponse{ResolvedValue: req.Extract(to), ResolvedReference: req.Reference}
		return rsp, errors.Wrapf(err, errGetManaged)
	}

	// The reference was not set, but a selector was. Select a reference.
	list := req.To.List
	if err := r.client.List(ctx, list, client.MatchingLabels(req.Selector.MatchLabels)); err != nil {
		return ResolutionResponse{}, errors.Wrapf(err, errListManaged)
	}

	for _, to := range list.GetItems() {
		if ControllersMustMatch(req.Selector) && !meta.HaveSameController(r.from, to) {
			continue
		}

		return ResolutionResponse{ResolvedValue: req.Extract(to), ResolvedReference: &v1alpha1.Reference{Name: to.GetName()}}, nil
	}

	// We couldn't resolve anything.
	return ResolutionResponse{}, nil
}

// ControllersMustMatch returns true if the supplied Selector requires that a
// reference be to a managed resource whose controller reference matches the
// referencing resource.
func ControllersMustMatch(s *v1alpha1.Selector) bool {
	if s == nil {
		return false
	}
	return s.MatchControllerRef != nil && *s.MatchControllerRef
}
