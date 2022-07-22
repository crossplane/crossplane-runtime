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

package kv

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/vault/api"

	"github.com/crossplane/crossplane-runtime/pkg/connection/store/vault/kv/fake"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	mountPath = "test-secrets/"

	secretName = "conn-unittests"
)

var (
	errBoom = errors.New("boom")
)

func TestV2ClientGet(t *testing.T) {
	type args struct {
		client LogicalClient
		path   string
	}
	type want struct {
		err error
		out *Secret
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
				path: secretName,
			},
			want: want{
				err: errors.Wrap(errBoom, errRead),
				out: NewSecret(nil, nil),
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
				path: secretName,
			},
			want: want{
				err: errors.New(ErrNotFound),
				out: NewSecret(nil, nil),
			},
		},
		"SuccessfulGetNoData": {
			reason: "Should successfully return secret from v2 KV engine even it only contains metadata.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return &api.Secret{
							// Using sample response here:
							// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
							Data: map[string]any{
								"metadata": map[string]any{
									"created_time": "2018-03-22T02:24:06.945319214Z",
									"custom_metadata": map[string]any{
										"owner":            "jdoe",
										"mission_critical": "false",
									},
									"deletion_time": "",
									"destroyed":     false,
								},
							},
						}, nil
					},
				},
				path: secretName,
			},
			want: want{
				out: NewSecret(nil, map[string]string{
					"owner":            "jdoe",
					"mission_critical": "false",
				}),
			},
		},
		"SuccessfulGetNoMetadata": {
			reason: "Should successfully return secret from v2 KV engine even it only contains data.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return &api.Secret{
							// Using sample response here:
							// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
							Data: map[string]any{
								"data": map[string]any{
									"foo": "bar",
								},
							},
						}, nil
					},
				},
				path: secretName,
			},
			want: want{
				out: NewSecret(map[string]string{
					"foo": "bar",
				}, nil),
			},
		},
		"SuccessfulGet": {
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
							Data: map[string]any{
								"data": map[string]any{
									"foo": "bar",
								},
								"metadata": map[string]any{
									"created_time": "2018-03-22T02:24:06.945319214Z",
									"custom_metadata": map[string]any{
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
				path: secretName,
			},
			want: want{
				out: NewSecret(map[string]string{
					"foo": "bar",
				}, map[string]string{
					"owner":            "jdoe",
					"mission_critical": "false",
				}),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := NewV2Client(tc.args.client, mountPath)

			s := Secret{}
			err := k.Get(tc.args.path, &s)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nv2Client.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.out, &s, cmpopts.IgnoreUnexported(Secret{})); diff != "" {
				t.Errorf("\n%s\nv2Client.Get(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestV2ClientApply(t *testing.T) {
	type args struct {
		client LogicalClient
		in     *Secret
		path   string

		ao []ApplyOption
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
				path: secretName,
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errRead), errGet),
			},
		},
		"ErrorWhileWritingData": {
			reason: "Should return a proper error if writing secret failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]string{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"custom_metadata": map[string]any{
										"foo": "bar",
										"baz": "qux",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1updated",
					"key3": "val3",
				}, map[string]string{
					"foo": "bar",
					"baz": "qux",
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteData),
			},
		},
		"ErrorWhileWritingMetadata": {
			reason: "Should return a proper error if writing secret metadata failed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]string{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"custom_metadata": map[string]any{
										"foo": "bar",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						return nil, errBoom
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1",
					"key2": "val2",
				}, map[string]string{
					"foo": "baz",
				}),
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteMetadata),
			},
		},
		"AlreadyUpToDate": {
			reason: "Should not perform a write if a v2 secret is already up to date.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							// Using sample response here:
							// https://www.vaultproject.io/api/secret/kv/kv-v2#sample-response-1
							Data: map[string]any{
								"data": map[string]any{
									"foo": "bar",
								},
								"metadata": map[string]any{
									"created_time": "2018-03-22T02:24:06.945319214Z",
									"custom_metadata": map[string]any{
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
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						return nil, errors.New("no write operation expected")
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"foo": "bar",
				}, map[string]string{
					"owner":            "jdoe",
					"mission_critical": "false",
				}),
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulCreate": {
			reason: "Should successfully create with new data if secret does not exists.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						// Vault logical client returns both error and secret as
						// nil if secret does not exist.
						return nil, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]any{
							"data": map[string]string{
								"key1": "val1",
								"key2": "val2",
							},
							"options": map[string]any{
								"cas": json.Number("0"),
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1",
					"key2": "val2",
				}, nil),
			},
			want: want{
				err: nil,
			},
		},
		"UpdateNotAllowed": {
			reason: "Should return not allowed error if update is not allowed.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]any{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"custom_metadata": map[string]any{
										"foo": "bar",
										"baz": "qux",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]any{
							"data": map[string]string{
								"key1": "val1updated",
								"key2": "val2",
								"key3": "val3",
							},
							"options": map[string]any{
								"cas": json.Number("2"),
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1updated",
					"key3": "val3",
				}, map[string]string{
					"foo": "bar",
					"baz": "qux",
				}),
				ao: []ApplyOption{
					AllowUpdateIf(func(current, desired *Secret) bool {
						return false
					}),
				},
			},
			want: want{
				err: resource.NewNotAllowed(errUpdateNotAllowed),
			},
		},
		"SuccessfulUpdateData": {
			reason: "Should successfully update by appending new data to existing ones.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]any{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"custom_metadata": map[string]any{
										"foo": "bar",
										"baz": "qux",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "data", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]any{
							"data": map[string]string{
								"key1": "val1updated",
								"key2": "val2",
								"key3": "val3",
							},
							"options": map[string]any{
								"cas": json.Number("2"),
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1updated",
					"key3": "val3",
				}, map[string]string{
					"foo": "bar",
					"baz": "qux",
				}),
				ao: []ApplyOption{
					AllowUpdateIf(func(current, desired *Secret) bool {
						return true
					}),
				},
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulAddMetadata": {
			reason: "Should successfully add new metadata.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]any{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "metadata", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]any{
							"custom_metadata": map[string]string{
								"foo": "bar",
								"baz": "qux",
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1",
					"key2": "val2",
				}, map[string]string{
					"foo": "bar",
					"baz": "qux",
				}),
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulUpdateMetadata": {
			reason: "Should successfully update metadata by overriding the existing ones.",
			args: args{
				client: &fake.LogicalClient{
					ReadFn: func(path string) (*api.Secret, error) {
						return &api.Secret{
							Data: map[string]any{
								"data": map[string]any{
									"key1": "val1",
									"key2": "val2",
								},
								"metadata": map[string]any{
									"custom_metadata": map[string]any{
										"old": "meta",
									},
									"version": json.Number("2"),
								},
							},
						}, nil
					},
					WriteFn: func(path string, data map[string]any) (*api.Secret, error) {
						if diff := cmp.Diff(filepath.Join(mountPath, "metadata", secretName), path); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						if diff := cmp.Diff(map[string]any{
							"custom_metadata": map[string]string{
								"foo": "bar",
								"baz": "qux",
							},
						}, data); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						return nil, nil
					},
				},
				path: secretName,
				in: NewSecret(map[string]string{
					"key1": "val1",
					"key2": "val2",
				}, map[string]string{
					"foo": "bar",
					"baz": "qux",
				}),
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := NewV2Client(tc.args.client, mountPath)

			err := k.Apply(tc.args.path, tc.args.in, tc.args.ao...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nv2Client.Apply(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestV2ClientDelete(t *testing.T) {
	type args struct {
		client LogicalClient
		path   string
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
				path: secretName,
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
				path: secretName,
			},
			want: want{
				err: nil,
			},
		},
		"SuccessfulDelete": {
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
				path: secretName,
			},
			want: want{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			k := NewV2Client(tc.args.client, mountPath)

			err := k.Delete(tc.args.path)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nv2Client.Get(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
