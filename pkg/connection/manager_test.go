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
							Name: "fake",
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
							Name: "fake",
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

func fakeStoreBuilderFn(ss fake.SecretStore) StoreBuilderFn {
	return func(_ context.Context, _ client.Client, cfg v1.SecretStoreConfig) (Store, error) {
		if *cfg.Type == fakeStore {
			return &ss, nil
		}
		return nil, errors.Errorf(errFmtUnknownSecretStore, *cfg.Type)
	}
}
