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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// NOTE(negz): FakeManagedList can't live with the other fakes in
// pkg/resource/fake as this would cause an import cycle.
type FakeManagedList struct {
	Items []Managed
}

func (l *FakeManagedList) GetItems() []Managed {
	return l.Items
}

func (l *FakeManagedList) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (l *FakeManagedList) DeepCopyObject() runtime.Object {
	out := &FakeManagedList{}
	j, err := json.Marshal(l)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

func TestReferenceFn(t *testing.T) {
	errBoom := errors.New("boom")
	name := "coolresource"

	now := metav1.Now()
	deleted := &fake.Managed{}
	deleted.SetDeletionTimestamp(&now)

	resolved := func(_ Managed) bool { return true }
	unresolved := func(_ Managed) bool { return false }
	selected := func(_ Managed) bool { return true }
	unselected := func(_ Managed) bool { return false }

	ready := &fake.Managed{}
	ready.SetConditions(v1alpha1.Available())

	resolve := func(from, _ Managed) { from.SetName(name) }
	sel := func(from, _ Managed) { from.SetName(name) }

	named := &fake.Managed{}
	named.SetName(name)

	matchController := true

	type args struct {
		ctx  context.Context
		c    client.Reader
		from Managed
	}

	type want struct {
		from Managed
		err  error
	}

	cases := map[string]struct {
		reason string
		fn     ReferenceFn
		args   args
		want   want
	}{
		"DefaultResolveFnFromWasDeleted": {
			reason: "The DefaultResolveFn should return early if the supplied managed resource was deleted",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{}, &fake.Managed{}, nil, nil),
			args: args{
				from: deleted,
			},
			want: want{
				from: deleted,
			},
		},
		"DefaultResolveFnAlreadyResolved": {
			reason: "The DefaultResolveFn should return early if the reference was already resolved",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{}, &fake.Managed{}, resolved, nil),
			args: args{
				from: &fake.Managed{},
			},
			want: want{
				from: &fake.Managed{},
			},
		},
		"DefaultResolveFnNotFoundError": {
			reason: "The DefaultResolveFn should return a ReferenceError if the referenced resource was not found",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{Name: name}, &fake.Managed{}, unresolved, nil),
			args: args{
				from: &fake.Managed{},
				c:    &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name))},
			},
			want: want{
				from: &fake.Managed{},
				err:  NewReferenceNotFoundError(name),
			},
		},
		"DefaultResolveFnGetError": {
			reason: "The DefaultResolveFn should return unknown errors encountered getting the referenced resource",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{}, &fake.Managed{}, unresolved, nil),
			args: args{
				from: &fake.Managed{},
				c:    &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			want: want{
				from: &fake.Managed{},
				err:  errors.Wrap(errBoom, errGetManaged),
			},
		},
		"DefaultResolveFnNotReadyError": {
			reason: "The DefaultResolveFn should return a ReferenceError if the referenced resource is not ready",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{Name: name}, &fake.Managed{}, unresolved, nil),
			args: args{
				from: &fake.Managed{},
				c:    &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			},
			want: want{
				from: &fake.Managed{},
				err:  NewReferenceNotReadyError(name),
			},
		},
		"DefaultResolveFnSuccess": {
			reason: "The DefaultResolveFn should not return an error if references were resolved",
			fn:     NewDefaultResolveFn(v1alpha1.Reference{Name: name}, ready, unresolved, resolve),
			args: args{
				from: &fake.Managed{},
				c:    &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			},
			want: want{
				from: named,
			},
		},
		"DefaultSelectFnFromWasDeleted": {
			reason: "The DefaultSelectFn should return early if the supplied managed resource was deleted",
			fn:     NewDefaultSelectFn(v1alpha1.Selector{}, &FakeManagedList{}, nil, nil),
			args: args{
				from: deleted,
			},
			want: want{
				from: deleted,
			},
		},
		"DefaultSelectFnAlreadySelectd": {
			reason: "The DefaultSelectFn should return early if the reference was already resolved",
			fn:     NewDefaultSelectFn(v1alpha1.Selector{}, &FakeManagedList{}, selected, nil),
			args: args{
				from: &fake.Managed{},
			},
			want: want{
				from: &fake.Managed{},
			},
		},
		"DefaultSelectFnListError": {
			reason: "The DefaultSelectFn should return unknown errors encountered getting the referenced resource",
			fn:     NewDefaultSelectFn(v1alpha1.Selector{}, &FakeManagedList{}, unselected, nil),
			args: args{
				from: &fake.Managed{},
				c:    &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			},
			want: want{
				from: &fake.Managed{},
				err:  errors.Wrap(errBoom, errListManaged),
			},
		},
		"DefaultSelectFnControllerMismatch": {
			reason: "The DefaultSelectFn should not select a managed resource without a matching controller reference",
			fn:     NewDefaultSelectFn(v1alpha1.Selector{MatchController: &matchController}, &FakeManagedList{}, unselected, sel),
			args: args{
				from: &fake.Managed{},
				c: &test.MockClient{MockList: test.NewMockListFn(nil, func(obj runtime.Object) error {
					obj.(*FakeManagedList).Items = []Managed{named}
					return nil
				})},
			},
			want: want{
				from: &fake.Managed{}, // named was not selected.
			},
		},

		"DefaultSelectFnSuccess": {
			reason: "The DefaultSelectFn should not return an error if references were selected",
			fn:     NewDefaultSelectFn(v1alpha1.Selector{}, &FakeManagedList{}, unselected, sel),
			args: args{
				from: &fake.Managed{},
				c: &test.MockClient{MockList: test.NewMockListFn(nil, func(obj runtime.Object) error {
					obj.(*FakeManagedList).Items = []Managed{named}
					return nil
				})},
			},
			want: want{
				from: named,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.fn(tc.args.ctx, tc.args.c, tc.args.from)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nReferenceFn(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.from, tc.args.from, test.EquateConditions()); diff != "" {
				t.Errorf("\n%s\nReferenceFn(...): -want from, +got from:\n%s", tc.reason, diff)
			}
		})
	}
}
