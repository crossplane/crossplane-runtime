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

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ManagedConnectionPropagator = &APIManagedConnectionPropagator{}
)

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

	namespace := "coolns"
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
								strings.Join([]string{AnnotationKeyPropagateToPrefix, string(uid)}, AnnotationDelimiter): strings.Join([]string{namespace, cmcsname}, AnnotationDelimiter),
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
								strings.Join([]string{AnnotationKeyPropagateToPrefix, "existing-uid"}, AnnotationDelimiter): "existing-namespace/existing-name",
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
								strings.Join([]string{AnnotationKeyPropagateToPrefix, "existing-uid"}, AnnotationDelimiter): "existing-namespace/existing-name",
								strings.Join([]string{AnnotationKeyPropagateToPrefix, string(uid)}, AnnotationDelimiter):    strings.Join([]string{namespace, cmcsname}, AnnotationDelimiter),
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
