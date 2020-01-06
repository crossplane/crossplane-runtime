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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedCreator              = &APIManagedCreator{}
	_ ManagedConnectionPropagator = &APIManagedConnectionPropagator{}
	_ Binder                      = &APIBinder{}
	_ Binder                      = &APIStatusBinder{}
	_ ClaimFinalizer              = &APIClaimFinalizer{}
	_ ManagedFinalizer            = &APIManagedFinalizer{}
	_ ManagedInitializer          = &ManagedNameAsExternalName{}
)

func TestCreate(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  Claim
		cs  Class
		mg  Managed
	}

	cmname := "coolclaim"
	csname := "coolclass"
	mgname := "coolmanaged"
	errBoom := errors.New("boom")

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"CreateManagedError": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				cs:  &fake.Class{},
				mg:  &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errCreateManaged),
		},
		"UpdateClaimError": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				cs:  &fake.Class{},
				mg:  &fake.Managed{},
			},
			want: errors.Wrap(errBoom, errUpdateClaim),
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := &fake.Managed{}
						want.SetName(mgname)
						want.SetClaimReference(&corev1.ObjectReference{
							Name:       cmname,
							APIVersion: fake.GVK(&fake.Claim{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Claim{}).Kind,
						})
						want.SetClassReference(&corev1.ObjectReference{
							Name:       csname,
							APIVersion: fake.GVK(&fake.Class{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Class{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &fake.Claim{}
						want.SetName(cmname)
						want.SetResourceReference(&corev1.ObjectReference{
							Name:       mgname,
							APIVersion: fake.GVK(&fake.Managed{}).GroupVersion().String(),
							Kind:       fake.GVK(&fake.Managed{}).Kind,
						})
						if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Class{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Name: cmname}},
				cs:  &fake.Class{ObjectMeta: metav1.ObjectMeta{Name: csname}},
				mg:  &fake.Managed{ObjectMeta: metav1.ObjectMeta{Name: mgname}},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedCreator(tc.fields.client, tc.fields.typer)
			err := api.Create(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Create(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}
func TestPropagateConnection(t *testing.T) {
	type fields struct {
		client client.Client
		typer  runtime.ObjectTyper
	}

	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	cmname := "coolclaim"
	mgname := "coolmanaged"
	uid := types.UID("definitely-a-uuid")
	cmcsname := "coolclaimsecret"
	mgcsname := "coolmanagedsecret"
	mgcsnamespace := "coolns"
	mgcsdata := map[string][]byte{"cool": []byte("data")}
	controller := true
	errBoom := errors.New("boom")

	cases := map[string]struct {
		fields fields
		args   args
		want   error
	}{
		"ClaimDoesNotWantConnectionSecret": {
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: nil,
		},
		"ManagedDoesNotExposeConnectionSecret": {
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: mgcsname},
					},
				},
				mg: &fake.Managed{},
			},
			want: nil,
		},
		"GetManagedSecretError": {
			fields: fields{
				client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: errors.Wrap(errBoom, errGetSecret),
		},
		"ClaimSecretConflictError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						switch n.Name {
						case cmcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        types.UID("some-other-uuid"),
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						case mgcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        uid,
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					ObjectMeta: metav1.ObjectMeta{Name: cmname},
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname, UID: uid},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"ClaimSecretUncontrolledError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						switch n.Name {
						case mgcsname:
							s := &corev1.Secret{}
							s.SetOwnerReferences([]metav1.OwnerReference{{
								UID:        uid,
								Controller: &controller,
							}})
							*o.(*corev1.Secret) = *s
						case cmcsname:
							// A secret without any owner references.
							*o.(*corev1.Secret) = corev1.Secret{}
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					ObjectMeta: metav1.ObjectMeta{Name: cmname},
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname, UID: uid},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: errors.Wrap(errors.New(errSecretConflict), errCreateOrUpdateSecret),
		},
		"ManagedSecretUpdateError": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {

						switch n.Name {
						case mgcsname:
							s := corev1.Secret{}
							s.SetNamespace(namespace)
							s.SetUID(uid)
							s.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})
							s.SetName(mgcsname)
							s.Data = mgcsdata
							*o.(*corev1.Secret) = s
						case cmcsname:
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						switch got.(metav1.Object).GetName() {
						case cmcsname:
						case mgcsname:
							return errBoom
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					ObjectMeta: metav1.ObjectMeta{Name: cmname},
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname, UID: uid},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: errors.Wrap(errBoom, errUpdateSecret),
		},
		"Successful": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := corev1.Secret{}
						s.SetNamespace(namespace)
						s.SetUID(uid)
						s.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})

						switch n.Name {
						case mgcsname:
							s.SetName(mgcsname)
							s.Data = mgcsdata
							*o.(*corev1.Secret) = s
						case cmcsname:
							s.SetName(cmcsname)
							*o.(*corev1.Secret) = s
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(namespace)
						want.SetUID(uid)
						want.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})
						want.Data = mgcsdata

						switch got.(metav1.Object).GetName() {
						case cmcsname:
							want.SetName(cmcsname)
							want.SetAnnotations(map[string]string{
								AnnotationKeyPropagateFromNamespace: namespace,
								AnnotationKeyPropagateFromName:      mgcsname,
								AnnotationKeyPropagateFromUID:       string(uid),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						case mgcsname:
							want.SetName(mgcsname)
							want.SetAnnotations(map[string]string{
								strings.Join([]string{AnnotationKeyPropagateToPrefix, string(uid)}, SlashDelimeter): strings.Join([]string{namespace, cmcsname}, SlashDelimeter),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: cmname, UID: uid},
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname, UID: uid},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: nil,
		},
		"SuccessfulWithExisting": {
			fields: fields{
				client: &test.MockClient{
					MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
						s := corev1.Secret{}
						s.SetNamespace(namespace)
						s.SetUID(uid)
						s.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})

						switch n.Name {
						case mgcsname:
							s.SetName(mgcsname)
							meta.AddAnnotations(&s, map[string]string{
								strings.Join([]string{AnnotationKeyPropagateToPrefix, "existing-uid"}, SlashDelimeter): "existing-namespace/existing-name",
							})
							s.Data = mgcsdata
							*o.(*corev1.Secret) = s
						case cmcsname:
							s.SetName(cmcsname)
							*o.(*corev1.Secret) = s
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := &corev1.Secret{}
						want.SetNamespace(namespace)
						want.SetUID(uid)
						want.SetOwnerReferences([]metav1.OwnerReference{{UID: uid, Controller: &controller}})
						want.Data = mgcsdata

						switch got.(metav1.Object).GetName() {
						case cmcsname:
							want.SetName(cmcsname)
							want.SetAnnotations(map[string]string{
								AnnotationKeyPropagateFromNamespace: namespace,
								AnnotationKeyPropagateFromName:      mgcsname,
								AnnotationKeyPropagateFromUID:       string(uid),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						case mgcsname:
							want.SetName(mgcsname)
							want.SetAnnotations(map[string]string{
								strings.Join([]string{AnnotationKeyPropagateToPrefix, "existing-uid"}, SlashDelimeter): "existing-namespace/existing-name",
								strings.Join([]string{AnnotationKeyPropagateToPrefix, string(uid)}, SlashDelimeter):    strings.Join([]string{namespace, cmcsname}, SlashDelimeter),
							})
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
						default:
							return errors.New("unexpected secret name")
						}
						return nil
					}),
				},
				typer: fake.SchemeWith(&fake.Claim{}, &fake.Managed{}),
			},
			args: args{
				ctx: context.Background(),
				cm: &fake.Claim{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: cmname, UID: uid},
					LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{
						Ref: &v1alpha1.LocalSecretReference{Name: cmcsname},
					},
				},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Name: mgname, UID: uid},
					ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{
						Ref: &v1alpha1.SecretReference{Namespace: mgcsnamespace, Name: mgcsname},
					},
				},
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIManagedConnectionPropagator(tc.fields.client, tc.fields.typer)
			err := api.PropagateConnection(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.PropagateConnection(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestBind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		cm  Claim
		mg  Managed
	}

	errBoom := errors.New("boom")
	externalName := "very-cool-external-resource"

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"UpdateClaimError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return nil
				case *fake.Claim:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithoutExternalName": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			typer:  fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithExternalName": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			typer:  fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIBinder(tc.client, tc.typer)
			err := api.Bind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Bind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Claim: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestStatusBind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		cm  Claim
		mg  Managed
	}

	errBoom := errors.New("boom")
	externalName := "very-cool-external-resource"

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"UpdateManagedError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
				switch obj.(type) {
				case *fake.Managed:
					return errBoom
				default:
					return errors.New("unexpected object kind")
				}
			})},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
				},
			},
		},
		"UpdateManagedStatusError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"UpdateClaimError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil, func(obj runtime.Object) error {
					switch obj.(type) {
					case *fake.Managed:
						return nil
					case *fake.Claim:
						return errBoom
					default:
						return errors.New("unexpected object kind")
					}
				}),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithoutExternalName": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg:  &fake.Managed{},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound}},
				mg: &fake.Managed{
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
		"SuccessfulWithExternalName": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			typer: fake.SchemeWith(&fake.Claim{}),
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{},
				mg: &fake.Managed{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
				},
			},
			want: want{
				err: nil,
				cm: &fake.Claim{
					ObjectMeta:    metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					BindingStatus: v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
				mg: &fake.Managed{
					ObjectMeta:      metav1.ObjectMeta{Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName}},
					ClaimReferencer: fake.ClaimReferencer{Ref: meta.ReferenceTo(&fake.Claim{}, fake.GVK(&fake.Claim{}))},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIStatusBinder(tc.client, tc.typer)
			err := api.Bind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Bind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Claim: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Bind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUnbind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"SuccessfulDelete": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
				MockDelete: test.NewMockDeleteFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"DeleteError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(nil),
				MockDelete: test.NewMockDeleteFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIBinder(tc.client, tc.typer)
			err := api.Unbind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Unbind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Unbind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
func TestStatusUnbind(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		typer  runtime.ObjectTyper
		args   args
		want   want
	}{
		"SuccessfulRetain": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"SuccessfulDelete": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				MockDelete:       test.NewMockDeleteFn(nil),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: nil,
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateError": {
			client: &test.MockClient{
				MockUpdate: test.NewMockUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManaged),
				mg: &fake.Managed{
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"UpdateStatusError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateManagedStatus),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimRetain},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
		"DeleteError": {
			client: &test.MockClient{
				MockUpdate:       test.NewMockUpdateFn(nil),
				MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
				MockDelete:       test.NewMockDeleteFn(errBoom),
			},
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseBound},
					ClaimReferencer: fake.ClaimReferencer{Ref: &corev1.ObjectReference{}},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteManaged),
				mg: &fake.Managed{
					Reclaimer:       fake.Reclaimer{Policy: v1alpha1.ReclaimDelete},
					BindingStatus:   v1alpha1.BindingStatus{Phase: v1alpha1.BindingPhaseReleased},
					ClaimReferencer: fake.ClaimReferencer{Ref: nil},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIStatusBinder(tc.client, tc.typer)
			err := api.Unbind(tc.args.ctx, tc.args.cm, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Unbind(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("api.Unbind(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClaimRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		cm  Claim
	}

	type want struct {
		err error
		cm  Claim
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateClaimError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIClaimFinalizer(tc.client, finalizer)
			err := api.RemoveFinalizer(tc.args.ctx, tc.args.cm)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Finalize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Finalize(...) Claim: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestManagedRemoveFinalizer(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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

func TestAPIClaimFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		cm  Claim
	}

	type want struct {
		err error
		cm  Claim
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		client client.Client
		args   args
		want   want
	}{
		"UpdateClaimError": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(errBoom)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateClaim),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
		"Successful": {
			client: &test.MockClient{MockUpdate: test.NewMockUpdateFn(nil)},
			args: args{
				ctx: context.Background(),
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{}}},
			},
			want: want{
				err: nil,
				cm:  &fake.Claim{ObjectMeta: metav1.ObjectMeta{Finalizers: []string{finalizer}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := NewAPIClaimFinalizer(tc.client, finalizer)
			err := api.AddFinalizer(tc.args.ctx, tc.args.cm)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("api.Initialize(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.cm, tc.args.cm, test.EquateConditions()); diff != "" {
				t.Errorf("api.Initialize(...) Claim: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestAPIManagedFinalizerAdder(t *testing.T) {
	finalizer := "veryfinal"

	type args struct {
		ctx context.Context
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
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
