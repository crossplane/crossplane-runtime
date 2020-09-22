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

package secret

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

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestReconciler(t *testing.T) {
	type args struct {
		m manager.Manager
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	mg := &fake.Managed{
		ObjectMeta: metav1.ObjectMeta{Name: "coolmanaged"},
		ConnectionSecretWriterTo: fake.ConnectionSecretWriterTo{Ref: &v1alpha1.SecretReference{
			Namespace: "coolns",
			Name:      "coolmanagedsecret",
		}},
	}
	tg := &fake.Target{
		ObjectMeta: metav1.ObjectMeta{Namespace: "coolns", Name: "cooltarget"},
		LocalConnectionSecretWriterTo: fake.LocalConnectionSecretWriterTo{Ref: &v1alpha1.LocalSecretReference{
			Name: "cooltargetsecret",
		}},
	}
	from := resource.ConnectionSecretFor(mg, fake.GVK(mg))
	to := resource.LocalConnectionSecretFor(tg, fake.GVK(tg))

	fromData := map[string][]byte{"cool": {1}}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		args args
		want want
	}{
		"ToNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
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
		"GetToError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
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
		"FromNotFound": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
								meta.AllowPropagation(from.DeepCopy(), o.(metav1.Object))
							case from.GetName():
								return kerrors.NewNotFound(schema.GroupResource{}, from.GetName())
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
		"GetFromError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
								meta.AllowPropagation(from.DeepCopy(), o.(metav1.Object))
							case from.GetName():
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
		"PropagationNotAllowedError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
								meta.AllowPropagation(from.DeepCopy(), o.(metav1.Object))
							case from.GetName():
								*o.(*corev1.Secret) = *from
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
				err:    errors.New(errPropagationNotAllowed),
			},
		},
		"UpdateToError": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
								s := to.DeepCopy()
								meta.AllowPropagation(from.DeepCopy(), s)
								*o.(*corev1.Secret) = *s
							case from.GetName():
								meta.AllowPropagation(o.(metav1.Object), to.DeepCopy())
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
		"SuccessfulSingle": {
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, n types.NamespacedName, o runtime.Object) error {
							switch n.Name {
							case to.GetName():
								s := to.DeepCopy()
								meta.AllowPropagation(from.DeepCopy(), s)
								*o.(*corev1.Secret) = *s
							case from.GetName():
								s := from.DeepCopy()
								s.Data = fromData
								meta.AllowPropagation(s, to.DeepCopy())
								*o.(*corev1.Secret) = *s
							default:
								return errors.New("unexpected secret name")
							}
							return nil
						},
						MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
							want := to.DeepCopy()
							want.Data = fromData
							meta.AllowPropagation(from.DeepCopy(), want)
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
			r := NewReconciler(tc.args.m)
			got, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Namespace: to.GetNamespace(), Name: to.GetName()}})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("r.Reconcile(...): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("r.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
