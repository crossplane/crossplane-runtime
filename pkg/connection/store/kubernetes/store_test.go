/*
 Copyright 2022 The Crossplane Authors.

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

package kubernetes

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	errBoom = errors.New("boom")

	fakeSecretName      = "fake"
	fakeSecretNamespace = "fake-namespace"

	fakeKV = map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	fakeLabels = map[string]string{
		"environment": "unit-test",
		"reason":      "testing",
	}

	fakeAnnotations = map[string]string{
		"some-annotation-key": "some-annotation-value",
	}

	storeTypeKubernetes = v1.SecretStoreKubernetes
)

func TestSecretStoreReadKeyValues(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		secret store.Secret
	}
	type want struct {
		result store.KeyValues
		err    error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"CannotGetSecret": {
			reason: "Should return a proper error if cannot get the secret",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
				secret: store.Secret{
					Name: fakeSecretName,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"SuccessfulRead": {
			reason: "Should return all key values after a success read",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = corev1.Secret{
								Data: fakeKV,
							}
							return nil
						}),
					},
				},
				secret: store.Secret{
					Name: "fake",
				},
			},
			want: want{
				result: store.KeyValues(fakeKV),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &SecretStore{
				client: tc.args.client,
			}

			got, err := ss.ReadKeyValues(context.Background(), tc.args.secret)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSecretStoreWriteKeyValues(t *testing.T) {
	type args struct {
		client           resource.ClientApplicator
		defaultNamespace string
		secret           store.Secret
		kv               store.KeyValues
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ApplyFailed": {
			reason: "Should return a proper error when cannot apply.",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						return errBoom
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(fakeKV),
			},
			want: want{
				err: errors.Wrap(errBoom, errApplySecret),
			},
		},
		"SecretAlreadyUpToDate": {
			reason: "Should not change secret if already up to date.",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						if diff := cmp.Diff(fakeConnectionSecret(withData(fakeKV)), obj.(*corev1.Secret)); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(fakeKV),
			},
			want: want{
				err: nil,
			},
		},
		"SecretUpdatedWithNewValue": {
			reason: "Should update value for an existing key if changed.",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						if diff := cmp.Diff(fakeConnectionSecret(withData(map[string][]byte{
							"existing-key": []byte("new-value"),
						})), obj.(*corev1.Secret)); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(map[string][]byte{
					"existing-key": []byte("new-value"),
				}),
			},
			want: want{
				err: nil,
			},
		},
		"SecretPatchedWithNewKey": {
			reason: "Should update existing secret additively if a new key added",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						if diff := cmp.Diff(fakeConnectionSecret(withData(map[string][]byte{
							"new-key": []byte("new-value"),
						})), obj.(*corev1.Secret)); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(map[string][]byte{
					"new-key": []byte("new-value"),
				}),
			},
			want: want{
				err: nil,
			},
		},
		"SecretCreatedWithData": {
			reason: "Should create a secret with all key values with default type.",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						if diff := cmp.Diff(fakeConnectionSecret(withData(fakeKV)), obj.(*corev1.Secret)); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(fakeKV),
			},
			want: want{
				err: nil,
			},
		},
		"SecretCreatedWithDataAndMetadata": {
			reason: "Should create a secret with all key values and provided metadata data.",
			args: args{
				client: resource.ClientApplicator{
					Applicator: resource.ApplyFn(func(ctx context.Context, obj client.Object, option ...resource.ApplyOption) error {
						if diff := cmp.Diff(fakeConnectionSecret(
							withData(fakeKV),
							withType(corev1.SecretTypeOpaque),
							withLabels(fakeLabels),
							withAnnotations(fakeAnnotations)), obj.(*corev1.Secret)); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					}),
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
					Metadata: v1.ConnectionSecretMetadata{
						Labels: map[string]string{
							"environment": "unit-test",
							"reason":      "testing",
						},
						Annotations: map[string]string{
							"some-annotation-key": "some-annotation-value",
						},
						Type: "Opaque",
					},
				},
				kv: store.KeyValues(fakeKV),
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &SecretStore{
				client:           tc.args.client,
				defaultNamespace: tc.args.defaultNamespace,
			}
			err := ss.WriteKeyValues(context.Background(), tc.args.secret, tc.args.kv)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.WriteKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSecretStoreDeleteKeyValues(t *testing.T) {
	type args struct {
		client           resource.ClientApplicator
		defaultNamespace string
		secret           store.Secret
		kv               store.KeyValues
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"CannotGetSecret": {
			reason: "Should return a proper error when it fails to get secret.",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetSecret),
			},
		},
		"SecretUpdatedWithRemainingKeys": {
			reason: "Should remove supplied keys from secret and update with remaining.",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = *fakeConnectionSecret(withData(fakeKV))
							return nil
						}),
						MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
							if diff := cmp.Diff(fakeConnectionSecret(withData(map[string][]byte{"key3": []byte("value3")})), obj.(*corev1.Secret)); diff != "" {
								t.Errorf("r: -want, +got:\n%s", diff)
							}
							return nil
						},
					},
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
				kv: store.KeyValues(map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				}),
			},
			want: want{
				err: nil,
			},
		},
		"CannotDeleteSecret": {
			reason: "Should return a proper error when it fails to delete secret.",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = *fakeConnectionSecret()
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteSecret),
			},
		},
		"SecretAlreadyDeleted": {
			reason: "Should not return error if secret already deleted.",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}),
					},
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
			},
			want: want{
				err: nil,
			},
		},
		"SecretDeletedNoKVSupplied": {
			reason: "Should delete the whole secret if no kv supplied as parameter.",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = *fakeConnectionSecret(withData(fakeKV))
							return nil
						}),
						MockDelete: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
							return nil
						},
					},
				},
				secret: store.Secret{
					Name:  fakeSecretName,
					Scope: fakeSecretNamespace,
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &SecretStore{
				client:           tc.args.client,
				defaultNamespace: tc.args.defaultNamespace,
			}
			err := ss.DeleteKeyValues(context.Background(), tc.args.secret, tc.args.kv)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.DeleteKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewSecretStore(t *testing.T) {
	type args struct {
		client resource.ClientApplicator
		cfg    v1.SecretStoreConfig
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"SuccessfulLocal": {
			reason: "Should return no error after successfully building local Kubernetes secret store",
			args: args{
				client: resource.ClientApplicator{},
				cfg: v1.SecretStoreConfig{
					Type:         &storeTypeKubernetes,
					DefaultScope: "test-ns",
				},
			},
			want: want{
				err: nil,
			},
		},
		"NoSecretWithRemoteKubeconfig": {
			reason: "Should fail properly if configured kubeconfig secret does not exist",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							return kerrors.NewNotFound(schema.GroupResource{}, "kube-conn")
						}),
					},
				},
				cfg: v1.SecretStoreConfig{
					Type:         &storeTypeKubernetes,
					DefaultScope: "test-ns",
					Kubernetes: &v1.KubernetesSecretStoreConfig{
						Auth: v1.KubernetesAuthConfig{
							Source: v1.CredentialsSourceSecret,
							CommonCredentialSelectors: v1.CommonCredentialSelectors{
								SecretRef: &v1.SecretKeySelector{
									SecretReference: v1.SecretReference{
										Name:      "kube-conn",
										Namespace: "test-ns",
									},
									Key: "kubeconfig",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.Wrap(kerrors.NewNotFound(schema.GroupResource{}, "kube-conn"), "cannot get credentials secret"), errExtractKubernetesAuthCreds), errBuildClient),
			},
		},
		"InvalidRestConfigForRemote": {
			reason: "Should fetch the configured kubeconfig and fail if it is not valid",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "kube-conn",
									Namespace: "test-ns",
								},
								Data: map[string][]byte{
									"kubeconfig": []byte(`
apiVersion: v1
kind: Config
malformed
`),
								},
							}
							return nil
						}),
					},
				},
				cfg: v1.SecretStoreConfig{
					Type:         &storeTypeKubernetes,
					DefaultScope: "test-ns",
					Kubernetes: &v1.KubernetesSecretStoreConfig{
						Auth: v1.KubernetesAuthConfig{
							Source: v1.CredentialsSourceSecret,
							CommonCredentialSelectors: v1.CommonCredentialSelectors{
								SecretRef: &v1.SecretKeySelector{
									SecretReference: v1.SecretReference{
										Name:      "kube-conn",
										Namespace: "test-ns",
									},
									Key: "kubeconfig",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.New("yaml: line 5: could not find expected ':'"), errBuildRestConfig), errBuildClient),
			},
		},
		"InvalidKubeconfigForRemote": {
			reason: "Should fetch the configured kubeconfig and fail if it is not valid",
			args: args{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							*obj.(*corev1.Secret) = corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "kube-conn",
									Namespace: "test-ns",
								},
								Data: map[string][]byte{
									"kubeconfig": []byte(`
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: TEST
    server: https://127.0.0.1:64695
  name: kind-kind
contexts:
- context:
    cluster: kind-kind
    namespace: crossplane-system
    user: kind-kind
  name: kind-kind
current-context: kind-kind
kind: Config
users:
- name: kind-kind
  user: {}
`),
								},
							}
							return nil
						}),
					},
				},
				cfg: v1.SecretStoreConfig{
					Type:         &storeTypeKubernetes,
					DefaultScope: "test-ns",
					Kubernetes: &v1.KubernetesSecretStoreConfig{
						Auth: v1.KubernetesAuthConfig{
							Source: v1.CredentialsSourceSecret,
							CommonCredentialSelectors: v1.CommonCredentialSelectors{
								SecretRef: &v1.SecretKeySelector{
									SecretReference: v1.SecretReference{
										Name:      "kube-conn",
										Namespace: "test-ns",
									},
									Key: "kubeconfig",
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("unable to load root certificates: unable to parse bytes as PEM block"), errBuildClient),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewSecretStore(context.Background(), tc.args.client, tc.args.cfg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewSecretStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

type secretOption func(*corev1.Secret)

func withType(t corev1.SecretType) secretOption {
	return func(s *corev1.Secret) {
		s.Type = t
	}
}

func withData(d map[string][]byte) secretOption {
	return func(s *corev1.Secret) {
		s.Data = d
	}
}

func withLabels(l map[string]string) secretOption {
	return func(s *corev1.Secret) {
		s.Labels = l
	}
}

func withAnnotations(a map[string]string) secretOption {
	return func(s *corev1.Secret) {
		s.Annotations = a
	}
}
func fakeConnectionSecret(opts ...secretOption) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeSecretName,
			Namespace: fakeSecretNamespace,
		},
		Type: resource.SecretTypeConnection,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}
