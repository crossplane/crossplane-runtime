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

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestAPIPatchingApplicator(t *testing.T) {
	errBoom := errors.New("boom")

	current := &object{Spec: "old"}
	current.SetName("foo")

	desired := &object{Spec: "new"}
	desired.SetName("foo")

	withRV := func(rv string, o client.Object) client.Object {
		cpy := *(o.(*object))
		cpy.SetResourceVersion(rv)
		return &cpy
	}

	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Thing"}
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "things"}
	singular := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "thing"}
	fakeRESTMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})
	fakeRESTMapper.AddSpecific(gvk, gvr, singular, meta.RESTScopeRoot)

	// for additive merge patch option test
	currentYAML := `
metadata:
  resourceVersion: "42"
a: old
b: old
`

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c: &test.MockClient{
				MockGet:                 test.NewMockGetFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("42", desired),
				// this is intentionally not a wrapped error because this comes from a client
				err: errBoom,
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:                 test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate:              test.NewMockCreateFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
				// this is intentionally not a wrapped error because this comes from a client
				err: errBoom,
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					o.SetResourceVersion("42")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o:  withRV("42", desired),
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   withRV("42", desired),
				err: errors.Wrapf(errBoom, "apply option failed for thing \"foo\""),
			},
		},
		"PatchError": {
			reason: "An error should be returned if we can't patch the object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					o.SetResourceVersion("42")
					return nil
				}),
				MockPatch:               test.NewMockPatchFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o:   withRV("42", desired),
				err: errBoom, // this is intentionally not a wrapped error because this comes from a client
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("1")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
			},
		},
		"Patched": {
			reason: "No error should be returned if we successfully patch an existing object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					o.SetResourceVersion("42")
					return nil
				}),
				MockPatch: test.NewMockPatchFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("43")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("43", desired),
			},
		},
		"GetConflictError": {
			reason: "No error should be returned if we successfully patch an existing object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					o.SetResourceVersion("100")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
				MockRESTMapper:          test.NewMockRESTMapperFn(fakeRESTMapper),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("42", desired),
				// this is intentionally not a wrapped error because this comes from a client
				err: kerrors.NewConflict(schema.GroupResource{Group: "example.com", Resource: "things"}, current.GetName(), errors.New(errOptimisticLock)),
			},
		},
		"PatchConflictError": {
			reason: "No error should be returned if we successfully patch an existing object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *current
					o.SetResourceVersion("42")
					return nil
				}),
				MockPatch:               test.NewMockPatchFn(kerrors.NewConflict(schema.GroupResource{Group: "example.com", Resource: "things"}, "foo", errors.New(errOptimisticLock))),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
				MockRESTMapper:          test.NewMockRESTMapperFn(fakeRESTMapper),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("42", desired),
				// this is intentionally not a wrapped error because this comes from a client
				err: kerrors.NewConflict(schema.GroupResource{Group: "example.com", Resource: "things"}, current.GetName(), errors.New(errOptimisticLock)),
			},
		},
		"AdditiveMergePatch": {
			reason: "No error with the old additive behaviour if desired",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					o.(*unstructured.Unstructured).Object = map[string]interface{}{}
					return yaml.Unmarshal([]byte(currentYAML), &o.(*unstructured.Unstructured).Object)
				}),
				MockPatch: func(_ context.Context, o client.Object, patch client.Patch, _ ...client.PatchOption) error {
					bs, err := patch.Data(o)
					if err != nil {
						return err
					}
					currentJSON, err := yaml.YAMLToJSON([]byte(currentYAML))
					if err != nil {
						return err
					}
					patched, err := jsonpatch.MergePatch(currentJSON, bs)
					if err != nil {
						return err
					}
					o.(*unstructured.Unstructured).Object = map[string]interface{}{}
					if err := json.Unmarshal(patched, &o.(*unstructured.Unstructured).Object); err != nil {
						return err
					}
					o.SetResourceVersion("43")
					return nil
				},
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
				MockRESTMapper:          test.NewMockRESTMapperFn(fakeRESTMapper),
			},
			args: args{
				o: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Thing",
						"metadata": map[string]interface{}{
							"resourceVersion": "42",
						},
						"b": "changed",
						"c": "added",
					},
				},
				ao: []ApplyOption{AdditiveMergePatchApplyOption},
			},
			want: want{
				o: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind": "Thing",
						"metadata": map[string]interface{}{
							"resourceVersion": "43",
						},
						"a": "old",
						"b": "changed",
						"c": "added",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIPatchingApplicator(tc.c)
			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestAPIUpdatingApplicator(t *testing.T) {
	errBoom := errors.New("boom")

	current := &object{Spec: "old"}
	current.SetName("foo")

	desired := &object{Spec: "new"}
	desired.SetName("foo")

	withRV := func(rv string, o client.Object) client.Object {
		cpy := *(o.(*object))
		cpy.SetResourceVersion(rv)
		return &cpy
	}

	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Thing"}
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "things"}
	singular := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "thing"}
	fakeRESTMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gvk.GroupVersion()})
	fakeRESTMapper.AddSpecific(gvk, gvr, singular, meta.RESTScopeRoot)

	type args struct {
		ctx context.Context
		o   client.Object
		ao  []ApplyOption
	}

	type want struct {
		o   client.Object
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Client
		args   args
		want   want
	}{
		"GetError": {
			reason: "An error should be returned if we can't get the object",
			c: &test.MockClient{
				MockGet:                 test.NewMockGetFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("42", desired),
				// this is intentionally not a wrapped error because this comes from a client
				err: errBoom,
			},
		},
		"CreateError": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet:                 test.NewMockGetFn(kerrors.NewNotFound(gvr.GroupResource(), desired.GetName())),
				MockCreate:              test.NewMockCreateFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: desired,
				// this is intentionally not a wrapped error because this comes from a client
				err: errBoom,
			},
		},
		"ApplyOptionError": {
			reason: "Any errors from an apply option should be returned",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("42")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o:  withRV("42", desired),
				ao: []ApplyOption{func(_ context.Context, _, _ runtime.Object) error { return errBoom }},
			},
			want: want{
				o:   withRV("42", desired),
				err: errors.Wrapf(errBoom, "apply option failed for thing \"foo\""),
			},
		},
		"UpdateError": {
			reason: "An error should be returned if we can't update the object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("42")
					return nil
				}),
				MockUpdate:              test.NewMockUpdateFn(errBoom),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("42", desired),
				// this is intentionally not a wrapped error because this comes from a client
				err: errBoom,
			},
		},
		"Created": {
			reason: "No error should be returned if we successfully create a new object",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(gvr.GroupResource(), desired.GetName())),
				MockCreate: test.NewMockCreateFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("1")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: desired,
			},
			want: want{
				o: withRV("1", desired),
			},
		},
		"Updated": {
			reason: "No error should be returned if we successfully update an existing object. If no ApplyOption is passed the existing should not be modified",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					*o.(*object) = *desired
					o.SetResourceVersion("42")
					return nil
				}),
				MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
					if diff := cmp.Diff(withRV("42", desired), o); diff != "" {
						t.Errorf("r: -want, +got:\n%s", diff)
					}
					o.SetResourceVersion("43")
					return nil
				}),
				MockGroupVersionKindFor: test.NewMockGroupVersionKindForFn(nil, gvk),
			},
			args: args{
				o: withRV("42", desired),
			},
			want: want{
				o: withRV("43", desired),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPIUpdatingApplicator(tc.c)
			err := a.Apply(tc.args.ctx, tc.args.o, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nApply(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, tc.args.o); diff != "" {
				t.Errorf("\n%s\nApply(...): -want, +got\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestManagedRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)
			err := api.RemoveFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		obj Object
	}

	type want struct {
		err error
		obj Object
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateObject),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				obj: &fake.Object{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIFinalizer(tc.client, finalizer)
			err := api.AddFinalizer(tc.args.ctx, tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.obj, tc.args.obj, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAdditiveMergePatchApplyOption(t *testing.T) {
	type args struct {
		current runtime.Object
		desired runtime.Object
	}
	type want struct {
		err     error
		current runtime.Object
		desired runtime.Object
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "equal unstructed",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
			},
			want: want{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
			},
		},
		{
			name: "overlapping unstructed",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
					"b":          "foo",
					"c":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
					"b":          "bar",
					"d":          "bar",
				}},
			},
			want: want{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
					"b":          "foo",
					"c":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
					"b":          "bar",
					"c":          "foo",
					"d":          "bar",
				}},
			},
		},
		{
			name: "equal typed",
			args: args{
				current: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
			},
			want: want{
				current: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
			},
		},
		{
			name: "overlapping typed",
			args: args{
				current: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "foo",
					"c": "foo",
				}}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "bar",
					"d": "bar",
				}}},
			},
			want: want{
				current: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "foo",
					"c": "foo",
				}}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "bar",
					"c": "foo",
					"d": "bar",
				}}},
			},
		},
		{
			name: "equal mixed",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"kind": "Thing",
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"a": "foo",
						},
					},
				}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
			},
			want: want{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"kind": "Thing",
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"a": "foo",
						},
					},
				}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
				}}},
			},
		},
		{
			name: "overlapping mixed",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"kind": "Thing",
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"a": "foo",
							"b": "foo",
							"c": "foo",
						},
					},
				}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "bar",
					"d": "bar",
				}}},
			},
			want: want{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"kind": "Thing",
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"a": "foo",
							"b": "foo",
							"c": "foo",
						},
					},
				}},
				desired: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
					"a": "foo",
					"b": "bar",
					"c": "foo",
					"d": "bar",
				}}},
			},
		},
		{
			name: "incomplete desired",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"a": "foo",
				}},
			},
			want: want{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
					"a":          "foo",
				}},
			},
		},
		{
			name: "different GVKs",
			args: args{
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "SomethingElse",
					"a":          "foo",
				}},
			},
			want: want{
				err: errors.New("cannot apply example.com/v1, Kind=SomethingElse to example.com/v1, Kind=Thing"),
				current: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "Thing",
				}},
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.com/v1",
					"kind":       "SomethingElse",
					"a":          "foo",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AdditiveMergePatchApplyOption(context.Background(), tt.args.current, tt.args.desired)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("AdditiveMergePatchApplyOption() error = %v, wantErr %v", err, tt.want.err)
			}
			if diff := cmp.Diff(tt.want.current, tt.args.current); diff != "" {
				t.Errorf("AdditiveMergePatchApplyOption()\ncurrent = %v\nwant    = %v\ndiff    = %s", tt.args.current, tt.want.current, diff)
			}
			if diff := cmp.Diff(tt.want.desired, tt.args.desired); diff != "" {
				t.Errorf("AdditiveMergePatchApplyOption()\ncurrent = %v\nwant    = %v\ndiff    = %s", tt.args.desired, tt.want.desired, diff)
			}
		})
	}
}
