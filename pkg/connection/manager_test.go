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

package connection

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/fake"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	resourcefake "github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	SecretStoreFake v1.SecretStoreType = "Fake"

	fakeConfig = "fake"
)

const (
	errBuildStore = "cannot build store"
)

var (
	fakeStore = SecretStoreFake

	errBoom = errors.New("boom")
)

func TestManagerConnectStore(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		p *v1.PublishConnectionDetailsTo
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ConfigNotFound": {
			reason: "We should return a proper error if referenced StoreConfig does not exist.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{}),
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
					},
				},
			},
			want: want{
				err: errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, fakeConfig), errGetStoreConfig),
			},
		},
		"BuildStoreError": {
			reason: "We should return any error encountered while building the Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: func(ctx context.Context, local client.Client, cfg v1.SecretStoreConfig) (Store, error) {
					return nil, errors.New(errBuildStore)
				},
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
					},
				},
			},
			want: want{
				err: errors.New(errBuildStore),
			},
		},
		"SuccessfulConnect": {
			reason: "We should not return an error when connected successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{}),
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
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
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			_, err := m.connectStore(context.Background(), tc.args.p)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.connectStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerPublishConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		conn managed.ConnectionDetails
		so   resource.ConnectionSecretOwner
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotToPublish": {
			reason: "We should return a proper error when publish to secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteStore),
			},
		},
		"SuccessfulPublish": {
			reason: "We should return no error when published successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
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
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			err := m.PublishConnection(context.Background(), tc.args.so, tc.args.conn)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.publishConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerUnpublishConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		conn managed.ConnectionDetails
		so   resource.ConnectionSecretOwner
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotUnpublish": {
			reason: "We should return a proper error when delete from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					DeleteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteFromStore),
			},
		},
		"SuccessfulUnpublish": {
			reason: "We should return no error when unpublished successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					DeleteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
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
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			err := m.UnpublishConnection(context.Background(), tc.args.so, tc.args.conn)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.unpublishConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerFetchConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		so resource.ConnectionSecretOwner
	}

	type want struct {
		conn managed.ConnectionDetails
		err  error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotFetch": {
			reason: "We should return a proper error when fetch from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return nil, errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadStore),
			},
		},
		"SuccessfulFetch": {
			reason: "We should return no error when fetched successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return store.KeyValues{
							"key1": []byte("val1"),
						}, nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				conn: map[string][]byte{
					"key1": []byte("val1"),
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			got, err := m.FetchConnection(context.Background(), tc.args.so)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.FetchConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, got); diff != "" {
				t.Errorf("\nReason: %s\nm.FetchConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerPropagateConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		to   resource.LocalConnectionSecretOwner
		from resource.ConnectionSecretOwner
	}

	type want struct {
		propagated bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetailsSource": {
			reason: "We should return no error if source resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				from: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"NoConnectionDetailsDestination": {
			reason: "We should return no error if destination resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				from: &resourcefake.MockConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
				to:   &resourcefake.MockLocalConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnectSource": {
			reason: "We should return any error encountered while connecting to Source Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, c store.KeyValues) error {
						return nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotFetch": {
			reason: "We should return a proper error when fetch from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return nil, errBoom
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadStore),
			},
		},
		"CannotConnectDestination": {
			reason: "We should return any error encountered while connecting to Destination Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == fakeConfig {
							*obj.(*fake.StoreConfig) = fake.StoreConfig{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeConfig,
								},
								Config: v1.SecretStoreConfig{
									Type: &fakeStore,
								},
							}
							return nil
						}

						return kerrors.NewNotFound(schema.GroupResource{}, "non-existing")
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return nil, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotPublish": {
			reason: "We should return any error encountered while publishing to Destination Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return nil, nil
					},
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, conn store.KeyValues) error {
						return errBoom
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteStore),
			},
		},
		"SuccessfulPropagate": {
			reason: "We should return no error when propagated successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, i store.Secret) (store.KeyValues, error) {
						return nil, nil
					},
					WriteKeyValuesFn: func(ctx context.Context, i store.Secret, conn store.KeyValues) error {
						return nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				propagated: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			got, err := m.PropagateConnection(context.Background(), tc.args.to, tc.args.from)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.PropagateConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.propagated, got); diff != "" {
				t.Errorf("\nReason: %s\nm.PropagateConnection(...): -want propagated, +got propagated:\n%s", tc.reason, diff)
			}
		})
	}
}

func fakeStoreBuilderFn(ss fake.SecretStore) StoreBuilderFn {
	return func(_ context.Context, _ client.Client, cfg v1.SecretStoreConfig) (Store, error) {
		if *cfg.Type == fakeStore {
			return &ss, nil
		}
		return nil, errors.Errorf(errFmtUnknownSecretStore, *cfg.Type)
	}
}
