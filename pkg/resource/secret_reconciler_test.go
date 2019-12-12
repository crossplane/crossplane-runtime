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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

func TestSecretPropagatingReconciler(t *testing.T) {
	type args struct {
		m manager.Manager
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	ns := "namespace"

	fromName := "from"
	fromUID := types.UID("from-uid")
	fromData := map[string][]byte{"cool": []byte("data")}

	toName := "to"
	toUID := types.UID("to-uid")

	errBoom := errors.New("boom")

	cases := map[string]struct {
		args args
		want want
	}{
		"FromNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case fromName:
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							default:
								return errors.New("unexpected secret name")
							}
						},
					},
				},
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"GetFromError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case fromName:
								return errBoom
							default:
								return errors.New("unexpected secret name")
							}
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"ToNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)
							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       string(toUID),
										},
									},
									Data: fromData,
								}
							case toName:
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
					},
				},
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"GetToError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)
							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       string(toUID),
										},
									},
									Data: fromData,
								}
							case toName:
								return errBoom
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"UnexpectedToUID": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)

							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       "some-other-uuid",
										},
									},
									Data: fromData,
								}
							case toName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      toName,
										UID:       toUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateFromNamespace: ns,
											AnnotationKeyPropagateFromName:      fromName,
											AnnotationKeyPropagateFromUID:       string(fromUID),
										},
									},
								}
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							return errors.New("called unexpectedly")
						}),
					},
				},
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"UnexpectedFromUID": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)

							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       string(toUID),
										},
									},
									Data: fromData,
								}
							case toName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      toName,
										UID:       toUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateFromNamespace: ns,
											AnnotationKeyPropagateFromName:      fromName,
											AnnotationKeyPropagateFromUID:       "some-other-uuid",
										},
									},
								}
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							return errors.New("called unexpectedly")
						}),
					},
				},
			},
			want: want{
				result: reconcile.Result{},
			},
		},
		"UpdateToError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)

							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       string(toUID),
										},
									},
									Data: fromData,
								}
							case toName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      toName,
										UID:       toUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateFromNamespace: ns,
											AnnotationKeyPropagateFromName:      fromName,
											AnnotationKeyPropagateFromUID:       string(fromUID),
										},
									},
								}
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							return errBoom
						}),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateSecret),
			},
		},
		"Successful": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							s := o.(*corev1.Secret)

							switch n.Name {
							case fromName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      fromName,
										UID:       fromUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateToNamespace: ns,
											AnnotationKeyPropagateToName:      toName,
											AnnotationKeyPropagateToUID:       string(toUID),
										},
									},
									Data: fromData,
								}
							case toName:
								*s = corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{
										Namespace: ns,
										Name:      toName,
										UID:       toUID,
										Annotations: map[string]string{
											AnnotationKeyPropagateFromNamespace: ns,
											AnnotationKeyPropagateFromName:      fromName,
											AnnotationKeyPropagateFromUID:       string(fromUID),
										},
									},
								}
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: ns,
									Name:      toName,
									UID:       toUID,
									Annotations: map[string]string{
										AnnotationKeyPropagateFromNamespace: ns,
										AnnotationKeyPropagateFromName:      fromName,
										AnnotationKeyPropagateFromUID:       string(fromUID),
									},
								},
								Data: fromData,
							}
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
			},
			want: want{
				result: reconcile.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewSecretPropagatingReconciler(tc.args.m)
			got, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: fromName}})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r.Reconcile(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("r.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
