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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedFinalizer   = &APIManagedFinalizer{}
	_ ManagedInitializer = &ManagedNameAsExternalName{}
)

func TestManagedRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedFinalizer(tc.client, finalizer)
			err := api.RemoveFinalizer(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.RemoveFinalizer(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIManagedFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedFinalizer(tc.client, finalizer)
			err := api.AddFinalizer(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedNameAsExternalName(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}

	type want struct {
		err error
		mg  resource.Managed
	}

	errBoom := errors.New("boom")
	testExternalName := "my-external-name"

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: testExternalName},
				}},
			},
		},
		"UpdateSuccessful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: testExternalName}},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: testExternalName},
				}},
			},
		},
		"UpdateNotNeeded": {
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: "some-name"},
				}},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{ObjectMeta: metav1.ObjectMeta{
					Name:        testExternalName,
					Annotations: map[string]string{meta.ExternalNameAnnotationKey: "some-name"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewManagedNameAsExternalName(tc.client)
			err := api.Initialize(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPISecretPublisher(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
		c   ConnectionDetails
	}

	mgname := "coolmanaged"
	mgcsnamespace := "coolnamespace"
	mgcsname := "coolmanagedsecret"
	mgcsdata := map[string][]byte{
		"cool":   []byte("data"),
		"cooler": []byte("notdata?"),
	}
	cddata := map[string][]byte{
		"cooler":  []byte("data"),
		"coolest": []byte("data"),
	}
	controller := true

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"ResourceDoesNotPublishSecret": {
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
			},
		},
		"ManagedSecretConflictError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetOwnerReferences([]metav1.OwnerReference{{
							UID:        types.UID("some-other-uuid"),
							Controller: &controller,
						}})
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
						Namespace: mgcsnamespace,
						Name:      mgcsname,
					}},
				},
				c: ConnectionDetails{},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"ManagedSecretUncontrolledError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						*o.(*corev1.Secret) = corev1.Secret{}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
						Namespace: mgcsnamespace,
						Name:      mgcsname,
					}},
				},
				c: ConnectionDetails{},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"SuccessfulCreate": {
			fields: fields{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(mgcsnamespace)
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
							Controller: &controller,
						}})
						want.Data = cddata
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
						Namespace: mgcsnamespace,
						Name:      mgcsname,
					}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
		"SuccessfulUpdateEmptyManagedSecret": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetNamespace(mgcsnamespace)
						s.SetName(mgcsname)
						s.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
							Controller: &controller,
						}})
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(mgcsnamespace)
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
							Controller: &controller,
						}})
						want.Data = cddata
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
						Namespace: mgcsnamespace,
						Name:      mgcsname,
					}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
		"SuccessfulUpdatePopulatedManagedSecret": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := &corev1.Secret{}
						s.SetNamespace(mgcsnamespace)
						s.SetName(mgcsname)
						s.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
							Controller: &controller,
						}})
						s.Data = mgcsdata
						*o.(*corev1.Secret) = *s
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(mgcsnamespace)
						want.SetName(mgcsname)
						want.SetOwnerReferences([]metav1.OwnerReference{{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
							Controller: &controller,
						}})
						want.Data = map[string][]byte{
							"cool":    []byte("data"),
							"cooler":  []byte("data"),
							"coolest": []byte("data"),
						}
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
						Namespace: mgcsnamespace,
						Name:      mgcsname,
					}},
				},
				c: ConnectionDetails(cddata),
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			a := NewAPISecretPublisher(tc.fields.client, tc.fields.typer)
			got := a.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
		})
	}
}
