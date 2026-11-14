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
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
)

var errBoom = errors.New("boom")

// scopeKey returns a canonical, order-independent key for a scope, so that
// fakes can index children by parent.
func scopeKey(s ExternalListScope) string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+s[k])
	}
	return strings.Join(parts, ",")
}

// pageBounds returns the half-open range [start, end) of one page of a
// total-length list, and the token for the following page ("" if none). The
// token encodes the index of the next item.
func pageBounds(total, pageSize int, pageToken string) (start, end int, next string) {
	if pageToken != "" {
		start, _ = strconv.Atoi(pageToken)
	}
	end = min(start+pageSize, total)
	if end < total {
		next = strconv.Itoa(end)
	}
	return start, end, next
}

// fakeScopedLister is an ExternalLister for a kind scoped by "zone", which is
// also an ExternalScopeLister. Its items are keyed by zone; its scopes are keyed
// by scopeKey(parent), with the top level keyed by "".
type fakeScopedLister struct {
	items    map[string][]ExternalListItem
	scopes   map[string][]ExternalListScope
	pageSize int
	// failPage, if non-zero, makes the List call for that 1-based page fail.
	failPage int
}

func (l *fakeScopedLister) List(_ context.Context, _ ProviderConfig, scope ExternalListScope, pageToken string) (ExternalListResult, error) {
	zone, ok := scope["zone"]
	if !ok {
		return ExternalListResult{}, ErrScopeRequired
	}
	all := l.items[zone]
	start, end, next := pageBounds(len(all), l.pageSize, pageToken)
	if l.failPage > 0 && start/l.pageSize+1 == l.failPage {
		return ExternalListResult{}, errBoom
	}
	return ExternalListResult{Items: all[start:end], NextPageToken: next}, nil
}

func (l *fakeScopedLister) ListScopes(_ context.Context, _ ProviderConfig, parent ExternalListScope, pageToken string) (ExternalScopeListResult, error) {
	all := l.scopes[scopeKey(parent)]
	start, end, next := pageBounds(len(all), l.pageSize, pageToken)
	return ExternalScopeListResult{Scopes: all[start:end], NextPageToken: next}, nil
}

// fakeUnscopedLister is an ExternalLister for a kind that is not scoped. It
// ignores the scope argument.
type fakeUnscopedLister struct {
	items []ExternalListItem
}

func (l *fakeUnscopedLister) List(_ context.Context, _ ProviderConfig, _ ExternalListScope, _ string) (ExternalListResult, error) {
	return ExternalListResult{Items: l.items}, nil
}

var (
	_ ExternalLister      = &fakeScopedLister{}
	_ ExternalScopeLister = &fakeScopedLister{}
	_ ExternalLister      = &fakeUnscopedLister{}
	_ ProviderConfig      = &fake.ProviderConfig{}
)

// listAll drains every page of ExternalLister.List, following the documented
// usage: call again with NextPageToken until it is empty. It returns the items
// gathered so far together with any error.
func listAll(l ExternalLister, scope ExternalListScope) ([]ExternalListItem, error) {
	var out []ExternalListItem
	var token string
	for {
		res, err := l.List(context.Background(), &fake.ProviderConfig{}, scope, token)
		if err != nil {
			return out, err
		}
		out = append(out, res.Items...)
		if res.NextPageToken == "" {
			return out, nil
		}
		token = res.NextPageToken
	}
}

// listAllScopes drains every page of ExternalScopeLister.ListScopes.
func listAllScopes(l ExternalScopeLister, parent ExternalListScope) ([]ExternalListScope, error) {
	var out []ExternalListScope
	var token string
	for {
		res, err := l.ListScopes(context.Background(), &fake.ProviderConfig{}, parent, token)
		if err != nil {
			return out, err
		}
		out = append(out, res.Scopes...)
		if res.NextPageToken == "" {
			return out, nil
		}
		token = res.NextPageToken
	}
}

// leafScopes walks the scope hierarchy depth-first from parent, returning the
// scopes that have no children. This is the documented way to enumerate every
// scope of a multi-level kind.
func leafScopes(l ExternalScopeLister, parent ExternalListScope) ([]ExternalListScope, error) {
	children, err := listAllScopes(l, parent)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		if len(parent) == 0 {
			return nil, nil
		}
		return []ExternalListScope{parent}, nil
	}
	var leaves []ExternalListScope
	for _, c := range children {
		sub, err := leafScopes(l, c)
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, sub...)
	}
	return leaves, nil
}

func TestExternalListerPagination(t *testing.T) {
	zoneA := []ExternalListItem{{ExternalName: "r1"}, {ExternalName: "r2"}, {ExternalName: "r3"}}
	l := &fakeScopedLister{items: map[string][]ExternalListItem{"a": zoneA}, pageSize: 2}

	got, err := listAll(l, ExternalListScope{"zone": "a"})
	if err != nil {
		t.Fatalf("listAll(): unexpected error: %v", err)
	}
	if diff := cmp.Diff(zoneA, got); diff != "" {
		t.Errorf("listAll(): -want, +got:\n%s", diff)
	}
}

func TestExternalListerScopeBoundsResults(t *testing.T) {
	l := &fakeScopedLister{
		items: map[string][]ExternalListItem{
			"a": {{ExternalName: "a1", Scope: ExternalListScope{"zone": "a"}}},
			"b": {{ExternalName: "b1", Scope: ExternalListScope{"zone": "b"}}},
		},
		pageSize: 10,
	}

	got, err := listAll(l, ExternalListScope{"zone": "a"})
	if err != nil {
		t.Fatalf("listAll(): unexpected error: %v", err)
	}
	want := []ExternalListItem{{ExternalName: "a1", Scope: ExternalListScope{"zone": "a"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("listAll(zone=a): -want, +got:\n%s", diff)
	}
}

func TestExternalListerScopeRequired(t *testing.T) {
	l := &fakeScopedLister{items: map[string][]ExternalListItem{"a": {{ExternalName: "a1"}}}, pageSize: 10}

	for name, scope := range map[string]ExternalListScope{
		"nil scope":   nil,
		"empty scope": {},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := listAll(l, scope)
			if !errors.Is(err, ErrScopeRequired) {
				t.Errorf("listAll(): got error %v, want %v", err, ErrScopeRequired)
			}
			if len(got) != 0 {
				t.Errorf("listAll(): got %d items without a scope, want 0", len(got))
			}
		})
	}
}

func TestExternalListerUnscopedIgnoresScope(t *testing.T) {
	want := []ExternalListItem{{ExternalName: "a"}}
	l := &fakeUnscopedLister{items: want}

	for name, scope := range map[string]ExternalListScope{
		"nil scope":        nil,
		"scope is ignored": {"zone": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := listAll(l, scope)
			if err != nil {
				t.Fatalf("listAll(): unexpected error: %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("listAll(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestExternalListerErrorMidPagination(t *testing.T) {
	// Five items at two per page: pages 1, 2 and 3. Page 2 fails.
	zoneA := []ExternalListItem{
		{ExternalName: "r1"}, {ExternalName: "r2"},
		{ExternalName: "r3"}, {ExternalName: "r4"},
		{ExternalName: "r5"},
	}
	l := &fakeScopedLister{items: map[string][]ExternalListItem{"a": zoneA}, pageSize: 2, failPage: 2}

	got, err := listAll(l, ExternalListScope{"zone": "a"})
	if !errors.Is(err, errBoom) {
		t.Fatalf("listAll(): got error %v, want %v", err, errBoom)
	}
	// Only the items from the page before the failure are returned.
	if diff := cmp.Diff(zoneA[:2], got); diff != "" {
		t.Errorf("listAll(): -want, +got:\n%s", diff)
	}
}

func TestExternalListerTypeAssertion(t *testing.T) {
	var scoped any = &fakeScopedLister{}
	if _, ok := scoped.(ExternalLister); !ok {
		t.Error("scoped kind: expected ExternalLister assertion to succeed")
	}
	if _, ok := scoped.(ExternalScopeLister); !ok {
		t.Error("scoped kind: expected ExternalScopeLister assertion to succeed")
	}

	var notDiscoverable any = struct{}{}
	if _, ok := notDiscoverable.(ExternalLister); ok {
		t.Error("client without List: expected ExternalLister assertion to fail")
	}
}

func TestExternalScopeListerTopLevelPagination(t *testing.T) {
	want := []ExternalListScope{{"account": "a1"}, {"account": "a2"}}
	l := &fakeScopedLister{
		scopes:   map[string][]ExternalListScope{"": want},
		pageSize: 1,
	}

	got, err := listAllScopes(l, nil)
	if err != nil {
		t.Fatalf("listAllScopes(): unexpected error: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("listAllScopes(): -want, +got:\n%s", diff)
	}
}

func TestExternalScopeListerNilAndEmptyParentAreEquivalent(t *testing.T) {
	want := []ExternalListScope{{"account": "a1"}}
	l := &fakeScopedLister{scopes: map[string][]ExternalListScope{"": want}, pageSize: 10}

	fromNil, err := listAllScopes(l, nil)
	if err != nil {
		t.Fatalf("listAllScopes(nil): unexpected error: %v", err)
	}
	fromEmpty, err := listAllScopes(l, ExternalListScope{})
	if err != nil {
		t.Fatalf("listAllScopes({}): unexpected error: %v", err)
	}
	if diff := cmp.Diff(fromNil, fromEmpty); diff != "" {
		t.Errorf("listAllScopes(nil) vs listAllScopes({}): -nil, +empty:\n%s", diff)
	}
	if diff := cmp.Diff(want, fromNil); diff != "" {
		t.Errorf("listAllScopes(nil): -want, +got:\n%s", diff)
	}
}

func TestExternalScopeListerChildScopesEmpty(t *testing.T) {
	l := &fakeScopedLister{scopes: map[string][]ExternalListScope{"": {{"zone": "a"}}}, pageSize: 10}

	got, err := listAllScopes(l, ExternalListScope{"zone": "a"})
	if err != nil {
		t.Fatalf("listAllScopes(): unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("listAllScopes(leaf): got %d scopes, want 0", len(got))
	}
}

func TestExternalScopeListerMultiLevelWalk(t *testing.T) {
	// Hierarchy: account -> zone. Only zones are leaves, and only zones can be
	// listed.
	l := &fakeScopedLister{
		scopes: map[string][]ExternalListScope{
			"":           {{"account": "a1"}, {"account": "a2"}},
			"account=a1": {{"account": "a1", "zone": "z1"}, {"account": "a1", "zone": "z2"}},
			"account=a2": {{"account": "a2", "zone": "z3"}},
		},
		items: map[string][]ExternalListItem{
			"z1": {{ExternalName: "r1"}, {ExternalName: "r2"}},
			"z2": {{ExternalName: "r3"}},
			"z3": {},
		},
		pageSize: 1,
	}

	leaves, err := leafScopes(l, nil)
	if err != nil {
		t.Fatalf("leafScopes(): unexpected error: %v", err)
	}
	wantLeaves := []ExternalListScope{
		{"account": "a1", "zone": "z1"},
		{"account": "a1", "zone": "z2"},
		{"account": "a2", "zone": "z3"},
	}
	if diff := cmp.Diff(wantLeaves, leaves); diff != "" {
		t.Fatalf("leafScopes(): -want, +got:\n%s", diff)
	}

	var all []ExternalListItem
	for _, leaf := range leaves {
		items, err := listAll(l, leaf)
		if err != nil {
			t.Fatalf("listAll(%v): unexpected error: %v", leaf, err)
		}
		all = append(all, items...)
	}
	wantItems := []ExternalListItem{{ExternalName: "r1"}, {ExternalName: "r2"}, {ExternalName: "r3"}}
	if diff := cmp.Diff(wantItems, all); diff != "" {
		t.Errorf("listing every leaf: -want, +got:\n%s", diff)
	}
}
