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

package client

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/vault/api"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/client/fake"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	mountPath = "test-secrets/"

	secretName = "conn-unittests"
)

var (
	errBoom = errors.New("boom")

	kvv1 = v1.VaultKVVersionV1
	kvv2 = v1.VaultKVVersionV2
)

func TestKVClientGet(t *testing.T) {
	type args struct {
		client  LogicalClient
		version *v1.VaultKVVersion
		path    string
	}
	type want struct {
		err error
		out *KVSecret
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileGettingSecret": {
			reason: "Should return a proper error if getting secret failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				err: errors.Wrap(errBoom, errRead),
				out: &KVSecret{},
			},
		},
		"SecretNotFound": {
			reason: "Should return a notFound error if secret does not exist.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						// Vault logical client returns both error and secret as
						// nil if secret does not exist.
						return nil, nil
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				err: errors.New(ErrNotFound),
				out: &KVSecret{},
			},
		},
		"SuccessfulGetFromV1": {
			reason: "Should successfully return secret from v1 KV engine.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return &api.Secret{
							Data: map[string]interface{}{
								"foo": "bar",
							},
						}, nil
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				out: &KVSecret{
					Data: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
		"SuccessfulGetFromV2": {
			reason: "Should successfully return secret from v2 KV engine.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return &api.Secret{
							// Using sample response here:
							// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"foo": "bar",
								},
								"metadata": map[string]interface{}{
									"created_time": "2018-03-22T02:24:06.945319214Z",
									"custom_metadata": map[string]interface{}{
										"owner":            "jdoe",
										"mission_critical": "false",
									},
									"deletion_time": "",
									"destroyed":     false,
									"version":       2,
								},
							},
						}, nil
					},
				},
				version: &kvv2,
				path:    secretName,
			},
			want: want{
				out: &KVSecret{
					Data: map[string]interface{}{
						"foo": "bar",
					},
					CustomMeta: map[string]interface{}{
						"owner":            "jdoe",
						"mission_critical": "false",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := NewAdditiveClient(tc.args.client, mountPath, WithVersion(tc.args.version))

			s := KVSecret{}
			err := k.Get(tc.args.path, &s)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nkvClient.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.out, &s, cmpopts.IgnoreUnexported(KVSecret{})); diff != "" {
				t.Errorf("\n%s\nkvClient.Get(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestKVClientApply(t *testing.T) {
	type args struct {
		client  LogicalClient
		version *v1.VaultKVVersion
		in      *KVSecret
		path    string
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileReadingSecret": {
			reason: "Should return a proper error if reading secret failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errRead), errGet),
			},
		},
		"ErrorWhileWritingV1Data": {
			reason: "Should return a proper error if writing secret failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"key1": "val1",
								"key2": "val2",
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv1,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1updated",
						"key3": "val3",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteData),
			},
		},
		"ErrorWhileWritingV2Data": {
			reason: "Should return a proper error if writing secret failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]interface{}{
									"custom_metadata": map[string]interface{}{
										"foo": "bar",
										"baz": "qux",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1updated",
						"key3": "val3",
					},
					CustomMeta: map[string]interface{}{
						"foo": "bar",
						"baz": "qux",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteData),
			},
		},
		"ErrorWhileWritingV2Metadata": {
			reason: "Should return a proper error if writing secret metadata failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]interface{}{
									"custom_metadata": map[string]interface{}{
										"foo": "bar",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1",
						"key2": "val2",
					},
					CustomMeta: map[string]interface{}{
						"foo": "baz",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteMetadata),
			},
		},
		"AlreadyUpToDateV1": {
			reason: "Should not perform a write if a v1 secret is already up to date.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"foo": "bar",
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						return nil, errors.New("no write operation expected")
					},
				},
				version: &kvv1,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"AlreadyUpToDateV2": {
			reason: "Should not perform a write if a v2 secret is already up to date.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							// Using sample response here:
							// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"foo": "bar",
								},
								"metadata": map[string]interface{}{
									"created_time": "2018-03-22T02:24:06.945319214Z",
									"custom_metadata": map[string]interface{}{
										"owner":            "jdoe",
										"mission_critical": "false",
									},
									"deletion_time": "",
									"destroyed":     false,
									"version":       2,
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						return nil, errors.New("no write operation expected")
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					CustomMeta: map[string]interface{}{
						"owner":            "jdoe",
						"mission_critical": "false",
					},
					Data: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulCreateV1": {
			reason: "Should successfully create with new data if secret does not exists.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						// Vault logical client returns both error and secret as
						// nil if secret does not exist.
						return nil, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"key1": "val1",
							"key2": "val2",
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv1,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1",
						"key2": "val2",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulCreateV2": {
			reason: "Should successfully create with new data if secret does not exists.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						// Vault logical client returns both error and secret as
						// nil if secret does not exist.
						return nil, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"data": map[string]interface{}{
								"key1": "val1",
								"key2": "val2",
							},
							"options": map[string]interface{}{
								"cas": json.Number("0"),
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1",
						"key2": "val2",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulUpdateV1": {
			reason: "Should successfully update by appending new data to existing ones.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"key1": "val1",
								"key2": "val2",
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"key1": "val1updated",
							"key2": "val2",
							"key3": "val3",
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv1,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1updated",
						"key3": "val3",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulUpdateV2Data": {
			reason: "Should successfully update by appending new data to existing ones.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]interface{}{
									"custom_metadata": map[string]interface{}{
										"foo": "bar",
										"baz": "qux",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"data": map[string]interface{}{
								"key1": "val1updated",
								"key2": "val2",
								"key3": "val3",
							},
							"options": map[string]interface{}{
								"cas": json.Number("2"),
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1updated",
						"key3": "val3",
					},
					CustomMeta: map[string]interface{}{
						"foo": "bar",
						"baz": "qux",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulAddV2Metadata": {
			reason: "Should successfully add new metadata.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]interface{}{
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "metadata", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"custom_metadata": map[string]interface{}{
								"foo": "bar",
								"baz": "qux",
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1",
						"key2": "val2",
					},
					CustomMeta: map[string]interface{}{
						"foo": "bar",
						"baz": "qux",
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulUpdateV2Metadata": {
			reason: "Should successfully update metadata by overriding the existing ones.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]interface{}{
								"data": map[string]interface{}{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]interface{}{
									"custom_metadata": map[string]interface{}{
										"old": "meta",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]interface{}) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "metadata", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]interface{}{
							"custom_metadata": map[string]interface{}{
								"foo": "bar",
								"baz": "qux",
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv2,
				path:    secretName,
				in: &KVSecret{
					Data: map[string]interface{}{
						"key1": "val1",
						"key2": "val2",
					},
					CustomMeta: map[string]interface{}{
						"foo": "bar",
						"baz": "qux",
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
			k := NewAdditiveClient(tc.args.client, mountPath, WithVersion(tc.args.version))

			err := k.Apply(tc.args.path, tc.args.in)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nkvClient.Apply(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestKVClientDelete(t *testing.T) {
	type args struct {
		client  LogicalClient
		version *v1.VaultKVVersion
		path    string
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileDeletingSecret": {
			reason: "Should return a proper error if deleting secret failed.",
			args: args{
				client: &fake.LogicalClient{
					DeleteFn: func(path string) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				err: errors.Wrap(errBoom, errDelete),
			},
		},
		"SecretAlreadyDeleted": {
			reason: "Should return success if secret already deleted.",
			args: args{
				client: &fake.LogicalClient{
					DeleteFn: func(path string) (*api.Secret, error) {
						// Vault logical client returns both error and secret as
						// nil if secret does not exist.
						return nil, nil
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulDeleteFromV1": {
			reason: "Should return no error after successful deletion of a v1 secret.",
			args: args{
				client: &fake.LogicalClient{
					DeleteFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return &api.Secret{
							Data: map[string]interface{}{
								"foo": "bar",
							},
						}, nil
					},
				},
				version: &kvv1,
				path:    secretName,
			},
			want: want{},
		},
		"SuccessfulDeleteFromV2": {
			reason: "Should return no error after successful deletion of a v2 secret.",
			args: args{
				client: &fake.LogicalClient{
					DeleteFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "metadata", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				version: &kvv2,
				path:    secretName,
			},
			want: want{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := NewAdditiveClient(tc.args.client, mountPath, WithVersion(tc.args.version))

			err := k.Delete(tc.args.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nkvClient.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
