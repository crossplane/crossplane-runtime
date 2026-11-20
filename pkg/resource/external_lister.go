/*
Copyright 2026 The Crossplane Authors.

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
	"errors"
)

// ErrScopeRequired is returned by ExternalLister.List when the kind is scoped
// and no scope was supplied.
var ErrScopeRequired = errors.New("external resource kind requires a scope")

// ExternalListScope identifies one set of external resources. Its keys are
// spec.forProvider JSON field names, for example {"zone": "abc"}. An empty
// scope denotes the top level of a kind that is not scoped.
type ExternalListScope map[string]string

// ExternalListItem is one external resource found by ExternalLister.List.
type ExternalListItem struct {
	// ExternalName matches the format expected by ExternalClient.Observe when
	// used as crossplane.io/external-name.
	ExternalName string

	// Scope holds the spec.forProvider fields, beyond the external name, that
	// Observe needs in order to locate the resource. Empty for kinds that are
	// not scoped.
	Scope ExternalListScope
}

// ExternalListResult is returned by ExternalLister.List.
type ExternalListResult struct {
	// Items are the external resources discovered.
	Items []ExternalListItem

	// NextPageToken, if non-empty, indicates there are more results. Callers
	// invoke List again with this token and the same scope.
	NextPageToken string
}

// ExternalLister enumerates external resources of a kind that exist in the
// provider's external system. It is an optional interface; providers that do
// not wish to support discovery may leave it unimplemented.
//
// Typical usage:
//
//	if lister, ok := externalClient.(ExternalLister); ok {
//		result, err := lister.List(ctx, providerConfig, nil, "")
//		// process result.Items
//		// if result.NextPageToken != "", call again with that token
//	}
type ExternalLister interface {
	// List enumerates external resources of the kind represented by the managed
	// resource definition, within the given scope. The ProviderConfig scopes the
	// search to a specific cloud account, project, credentials, or similar
	// context.
	//
	// A kind that is not scoped ignores the scope argument. A kind that requires
	// a scope and is called without one returns ErrScopeRequired.
	//
	// Callers must check NextPageToken and invoke List again until it is empty.
	// Implementations should respect reasonable rate limiting and timeouts to
	// avoid overwhelming the external API.
	//
	// pageToken is empty on the first call, and contains the value from the
	// previous call's result.NextPageToken on subsequent calls.
	List(ctx context.Context, pc ProviderConfig, scope ExternalListScope, pageToken string) (ExternalListResult, error)
}

// ExternalScopeListResult is returned by ExternalScopeLister.ListScopes.
type ExternalScopeListResult struct {
	// Scopes are the child scopes of the parent that was requested.
	Scopes []ExternalListScope

	// NextPageToken, if non-empty, indicates there are more results.
	NextPageToken string
}

// ExternalScopeLister enumerates the scopes of a kind, so that callers can list
// each scope in turn. It is an optional companion to ExternalLister for kinds
// that are scoped.
type ExternalScopeLister interface {
	// ListScopes enumerates the child scopes of parent. A nil or empty parent
	// enumerates the top-level scopes. Hierarchies with more than one level are
	// enumerated by calling ListScopes with a returned scope as the parent.
	ListScopes(ctx context.Context, pc ProviderConfig, parent ExternalListScope, pageToken string) (ExternalScopeListResult, error)
}
