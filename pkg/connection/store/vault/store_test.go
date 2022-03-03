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

package vault

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	kvclient "github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/client"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/fake"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	parentPathDefault = "crossplane-system"

	secretName = "conn-unittests"
)

var (
	errBoom = errors.New("boom")
)

func TestSecretStoreReadKeyValues(t *testing.T) {
	type args struct {
		client            KVClient
		defaultParentPath string
		secret            store.Secret
	}
	type want struct {
		out managed.ConnectionDetails
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileGetting": {
			reason: "Should return a proper error if secret cannot be obtained",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						return errBoom
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name:     secretName,
					Metadata: nil,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"SuccessfulGetWithDefaultScope": {
			reason: "Should return key values from a secret with default scope",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						if diff := cmp.Diff(filepath.Join(parentPathDefault, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
						}
						return nil
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name:     secretName,
					Metadata: nil,
				},
			},
			want: want{
				out: managed.ConnectionDetails{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
				err: nil,
			},
		},
		"SuccessfulGetWithCustomScope": {
			reason: "Should return key values from a secret with custom scope",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						if diff := cmp.Diff(filepath.Join("another-scope", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
						}
						return nil
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name:     secretName,
					Scope:    "another-scope",
					Metadata: nil,
				},
			},
			want: want{
				out: managed.ConnectionDetails{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &SecretStore{
				client:            tc.args.client,
				defaultParentPath: tc.args.defaultParentPath,
			}
			got, err := ss.ReadKeyValues(context.Background(), tc.secret)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSecretStoreWriteKeyValues(t *testing.T) {
	type args struct {
		client            KVClient
		defaultParentPath string
		secret            store.Secret
		in                managed.ConnectionDetails
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrWhileApplying": {
			reason: "Should successfully write key values",
			args: args{
				client: &fake.KVClient{
					ApplyFn: func(path string, secret *kvclient.KVSecret) error {
						return errBoom
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name: secretName,
				},
				in: managed.ConnectionDetails{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApply),
			},
		},
		"SuccessfulWrite": {
			reason: "Should successfully write key values",
			args: args{
				client: &fake.KVClient{
					ApplyFn: func(path string, secret *kvclient.KVSecret) error {
						if diff := cmp.Diff(filepath.Join(parentPathDefault, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
						}, secret.Data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name: secretName,
				},
				in: managed.ConnectionDetails{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulWriteWithMetadata": {
			reason: "Should successfully write key values",
			args: args{
				client: &fake.KVClient{
					ApplyFn: func(path string, secret *kvclient.KVSecret) error {
						if diff := cmp.Diff(filepath.Join(parentPathDefault, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
						}, secret.Data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"foo": "bar",
						}, secret.CustomMeta); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					},
				},
				defaultParentPath: parentPathDefault,
				secret: store.Secret{
					Name: secretName,
					Metadata: &v1.ConnectionSecretMetadata{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				in: managed.ConnectionDetails{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
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
				client:            tc.args.client,
				defaultParentPath: tc.args.defaultParentPath,
			}
			err := ss.WriteKeyValues(context.Background(), tc.args.secret, tc.args.in)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSecretStoreDeleteKeyValues(t *testing.T) {
	type args struct {
		client            KVClient
		defaultParentPath string
		secret            store.Secret
		in                managed.ConnectionDetails
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorGettingSecret": {
			reason: "Should return a proper error if getting secret fails.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						return errBoom
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"AlreadyDeleted": {
			reason: "Should return no error if connection secret already deleted.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						return errors.New(kvclient.ErrNotFound)
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
			},
			want: want{
				err: nil,
			},
		},
		"DeletesSecretIfNoKVProvided": {
			reason: "Should delete whole secret if no kv provided as input",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
							"key3": "val3",
						}
						return nil
					},
					DeleteFn: func(path string) error {
						return nil
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
			},
			want: want{
				err: nil,
			},
		},
		"ErrorUpdatingSecretWithRemaining": {
			reason: "Should return a proper error if updating secret with remaining keys fails.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
							"key3": "val3",
						}
						return nil
					},
					ApplyFn: func(path string, secret *kvclient.KVSecret) error {
						return errBoom
					},
					DeleteFn: func(path string) error {
						return errors.New("unexpected delete call")
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
				in: map[string][]byte{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApply),
			},
		},
		"UpdatesSecretByRemovingProvidedKeys": {
			reason: "Should only delete provided keys and should not delete secret if kv provided as input.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
							"key3": "val3",
						}
						return nil
					},
					ApplyFn: func(path string, secret *kvclient.KVSecret) error {
						if diff := cmp.Diff(map[string]interface{}{
							"key3": "val3",
						}, secret.Data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil
					},
					DeleteFn: func(path string) error {
						return errors.New("unexpected delete call")
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
				in: map[string][]byte{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
				},
			},
			want: want{
				err: nil,
			},
		},
		"ErrorDeletingSecret": {
			reason: "Should return a proper error if deleting the secret after no keys left fails.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
							"key3": "val3",
						}
						return nil
					},
					DeleteFn: func(path string) error {
						return errBoom
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
				in: map[string][]byte{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
					"key3": []byte("val3"),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDelete),
			},
		},
		"DeletesSecretIfNoKeysLeft": {
			reason: "Should delete the secret if no keys left.",
			args: args{
				client: &fake.KVClient{
					GetFn: func(path string, secret *kvclient.KVSecret) error {
						secret.Data = map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
							"key3": "val3",
						}
						return nil
					},
					DeleteFn: func(path string) error {
						return nil
					},
				},
				secret: store.Secret{
					Name: secretName,
				},
				in: map[string][]byte{
					"key1": []byte("val1"),
					"key2": []byte("val2"),
					"key3": []byte("val3"),
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
				client:            tc.args.client,
				defaultParentPath: tc.args.defaultParentPath,
			}
			err := ss.DeleteKeyValues(context.Background(), tc.args.secret, tc.args.in)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewSecretStore(t *testing.T) {
	type args struct {
		kube client.Client
		cfg  v1.SecretStoreConfig
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"InvalidAuthConfig": {
			reason: "Should return a proper error if vault auth configuration is not valid.",
			args: args{
				cfg: v1.SecretStoreConfig{
					Vault: &v1.VaultSecretStoreConfig{
						Auth: v1.VaultAuthConfig{
							Method: v1.VaultAuthToken,
							Token:  nil,
						},
					},
				},
			},
			want: want{
				err: errors.New(errNoTokenProvided),
			},
		},
		"NoTokenSecret": {
			reason: "Should return a proper error if configured vault token secret does not exist.",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, "vault-token")
					}),
				},
				cfg: v1.SecretStoreConfig{
					Vault: &v1.VaultSecretStoreConfig{
						Auth: v1.VaultAuthConfig{
							Method: v1.VaultAuthToken,
							Token: &v1.VaultAuthTokenConfig{
								Source: v1.CredentialsSourceSecret,
								CommonCredentialSelectors: v1.CommonCredentialSelectors{
									SecretRef: &v1.SecretKeySelector{
										SecretReference: v1.SecretReference{
											Name:      "vault-token",
											Namespace: "crossplane-system",
										},
										Key: "token",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(kerrors.NewNotFound(schema.GroupResource{}, "vault-token"), "cannot get credentials secret"), errExtractToken),
			},
		},
		"SuccessfulStore": {
			reason: "Should return no error after building store successfully.",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						*obj.(*corev1.Secret) = corev1.Secret{
							Data: map[string][]byte{
								"token": []byte("t0ps3cr3t"),
							},
						}
						return nil
					}),
				},
				cfg: v1.SecretStoreConfig{
					Vault: &v1.VaultSecretStoreConfig{
						Auth: v1.VaultAuthConfig{
							Method: v1.VaultAuthToken,
							Token: &v1.VaultAuthTokenConfig{
								Source: v1.CredentialsSourceSecret,
								CommonCredentialSelectors: v1.CommonCredentialSelectors{
									SecretRef: &v1.SecretKeySelector{
										SecretReference: v1.SecretReference{
											Name:      "vault-token",
											Namespace: "crossplane-system",
										},
										Key: "token",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewSecretStore(context.Background(), tc.args.kube, tc.args.cfg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewSecretStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
