/*
Copyright 2020 The Crossplane Authors.

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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

func TestExtractEnv(t *testing.T) {
	credentials := []byte("supersecretcreds")

	type args struct {
		e     EnvLookupFn
		creds xpv2.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EnvVarSuccess": {
			reason: "Successful extraction of credentials from environment variable",
			args: args{
				e: func(string) string { return string(credentials) },
				creds: xpv2.CommonCredentialSelectors{
					Env: &xpv2.EnvSelector{
						Name: "SECRET_CREDS",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"EnvVarFail": {
			reason: "Failed extraction of credentials from environment variable",
			args: args{
				e: func(string) string { return string(credentials) },
			},
			want: want{
				err: errors.New(errExtractEnv),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractEnv(context.TODO(), tc.args.e, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractEnv(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractEnv(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestExtractFs(t *testing.T) {
	credentials := []byte("supersecretcreds")
	mockFs := afero.NewMemMapFs()
	f, _ := mockFs.Create("credentials.txt")
	f.Write(credentials)
	f.Close()

	type args struct {
		fs    afero.Fs
		creds xpv2.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FsSuccess": {
			reason: "Successful extraction of credentials from filesystem",
			args: args{
				fs: mockFs,
				creds: xpv2.CommonCredentialSelectors{
					Fs: &xpv2.FsSelector{
						Path: "credentials.txt",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"FsFailure": {
			reason: "Failed extraction of credentials from filesystem",
			args: args{
				fs: mockFs,
			},
			want: want{
				err: errors.New(errExtractFs),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractFs(context.TODO(), tc.args.fs, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractFs(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractFs(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestExtractSecret(t *testing.T) {
	errBoom := errors.New("boom")
	credentials := []byte("supersecretcreds")

	type args struct {
		client client.Client
		creds  xpv2.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SecretSuccess": {
			reason: "Successful extraction of credentials from Secret",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						s, _ := o.(*corev1.Secret)
						s.Data = map[string][]byte{
							"creds": credentials,
						}

						return nil
					}),
				},
				creds: xpv2.CommonCredentialSelectors{
					SecretRef: &xpv2.SecretKeySelector{
						SecretReference: xpv2.SecretReference{
							Name:      "super",
							Namespace: "secret",
						},
						Key: "creds",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"SecretFailureNotDefined": {
			reason: "Failed extraction of credentials from Secret when key not defined",
			args:   args{},
			want: want{
				err: errors.New(errExtractSecretKey),
			},
		},
		"SecretFailureGet": {
			reason: "Failed extraction of credentials from Secret when client fails",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(client.Object) error {
						return errBoom
					}),
				},
				creds: xpv2.CommonCredentialSelectors{
					SecretRef: &xpv2.SecretKeySelector{
						SecretReference: xpv2.SecretReference{
							Name:      "super",
							Namespace: "secret",
						},
						Key: "creds",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCredentialsSecret),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractSecret(context.TODO(), tc.args.client, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractSecret(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractSecret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestTrackLegacy(t *testing.T) {
	errBoom := errors.New("boom")
	name := "provisional"

	type fields struct {
		c  Applicator
		of LegacyProviderConfigUsage
	}

	type args struct {
		ctx context.Context
		mg  LegacyManaged
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"MissingRef": {
			reason: "An error that satisfies IsMissingReference should be returned if the managed resource has no provider config reference",
			fields: fields{
				of: &fake.LegacyProviderConfigUsage{},
			},
			args: args{
				mg: &fake.LegacyManaged{},
			},
			want: missingRefError{errors.New(errMissingPCRef)},
		},
		"NopUpdate": {
			reason: "No error should be returned if the apply fails because it would be a no-op",
			fields: fields{
				c: ApplyFn(func(ctx context.Context, _ client.Object, ao ...ApplyOption) error {
					for _, fn := range ao {
						// Exercise the MustBeControllableBy and AllowUpdateIf
						// ApplyOptions. The former should pass because the
						// current object has no controller ref. The latter
						// should return an error that satisfies IsNotAllowed
						// because the current object has the same PC ref as the
						// new one we would apply.
						current := &fake.LegacyProviderConfigUsage{
							RequiredProviderConfigReferencer: fake.RequiredProviderConfigReferencer{
								Ref: xpv2.Reference{Name: name},
							},
						}
						current.SetFinalizers([]string{ProviderConfigUsageFinalizer})
						if err := fn(ctx, current, nil); err != nil {
							return err
						}
					}

					return errBoom
				}),
				of: &fake.LegacyProviderConfigUsage{},
			},
			args: args{
				mg: &fake.LegacyManaged{
					LegacyProviderConfigReferencer: fake.LegacyProviderConfigReferencer{
						Ref: &xpv2.Reference{Name: name},
					},
				},
			},
			want: nil,
		},
		"ApplyError": {
			reason: "Errors applying the ProviderConfigUsage should be returned",
			fields: fields{
				c: ApplyFn(func(_ context.Context, o client.Object, _ ...ApplyOption) error {
					if !meta.FinalizerExists(o.(LegacyProviderConfigUsage), ProviderConfigUsageFinalizer) {
						t.Error("ProviderConfigUsage finalizer was not added")
					}
					return errBoom
				}),
				of: &fake.LegacyProviderConfigUsage{},
			},
			args: args{
				mg: &fake.LegacyManaged{
					LegacyProviderConfigReferencer: fake.LegacyProviderConfigReferencer{
						Ref: &xpv2.Reference{Name: name},
					},
				},
			},
			want: errors.Wrap(errBoom, errApplyPCU),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ut := &LegacyProviderConfigUsageTracker{c: tc.fields.c, of: tc.fields.of}

			got := ut.Track(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nut.Track(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestTrackModern(t *testing.T) {
	errBoom := errors.New("boom")
	name := "provisional"

	type fields struct {
		c  Applicator
		of TypedProviderConfigUsage
	}

	type args struct {
		ctx context.Context
		mg  ModernManaged
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"MissingRef": {
			reason: "An error that satisfies IsMissingReference should be returned if the managed resource has no provider config reference",
			fields: fields{
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.ModernManaged{},
			},
			want: missingRefError{errors.New(errMissingPCRef)},
		},
		"NopUpdate": {
			reason: "No error should be returned if the apply fails because it would be a no-op",
			fields: fields{
				c: ApplyFn(func(ctx context.Context, _ client.Object, ao ...ApplyOption) error {
					for _, fn := range ao {
						// Exercise the MustBeControllableBy and AllowUpdateIf
						// ApplyOptions. The former should pass because the
						// current object has no controller ref. The latter
						// should return an error that satisfies IsNotAllowed
						// because the current object has the same PC ref as the
						// new one we would apply.
						current := &fake.ProviderConfigUsage{
							RequiredTypedProviderConfigReferencer: fake.RequiredTypedProviderConfigReferencer{
								Ref: xpv2.ProviderConfigReference{Name: name, Kind: "ProviderConfig"},
							},
						}
						current.SetFinalizers([]string{ProviderConfigUsageFinalizer})
						if err := fn(ctx, current, nil); err != nil {
							return err
						}
					}

					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.ModernManaged{
					TypedProviderConfigReferencer: fake.TypedProviderConfigReferencer{
						Ref: &xpv2.ProviderConfigReference{Name: name, Kind: "ProviderConfig"},
					},
				},
			},
			want: nil,
		},
		"ApplyError": {
			reason: "Errors applying the ProviderConfigUsage should be returned",
			fields: fields{
				c: ApplyFn(func(_ context.Context, o client.Object, _ ...ApplyOption) error {
					if !meta.FinalizerExists(o.(TypedProviderConfigUsage), ProviderConfigUsageFinalizer) {
						t.Error("ProviderConfigUsage finalizer was not added")
					}
					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.ModernManaged{
					TypedProviderConfigReferencer: fake.TypedProviderConfigReferencer{
						Ref: &xpv2.ProviderConfigReference{Name: name, Kind: "ProviderConfig"},
					},
				},
			},
			want: errors.Wrap(errBoom, errApplyPCU),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ut := &ProviderConfigUsageTracker{c: tc.fields.c, of: tc.fields.of}

			got := ut.Track(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nut.Track(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestUntrack(t *testing.T) {
	uid := types.UID("some-uid")
	namespace := "some-namespace"

	for name, tc := range map[string]struct {
		reason  string
		cleaner func(client.Client) ProviderConfigUsageCleaner
		mg      func() Managed
		wantNS  string
	}{
		"Legacy": {
			reason: "The cluster-scoped tracker must look its usage up by the managed resource UID without a namespace.",
			cleaner: func(c client.Client) ProviderConfigUsageCleaner {
				return NewLegacyProviderConfigUsageTracker(c, &fake.LegacyProviderConfigUsage{})
			},
			// The namespace of a cluster-scoped managed resource is ignored.
			mg: func() Managed { return &fake.LegacyManaged{} },
		},
		"Modern": {
			reason: "The namespaced tracker must look its usage up by the managed resource UID in its namespace.",
			cleaner: func(c client.Client) ProviderConfigUsageCleaner {
				return NewProviderConfigUsageTracker(c, &fake.ProviderConfigUsage{})
			},
			mg:     func() Managed { return &fake.ModernManaged{} },
			wantNS: namespace,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := test.NewMockClient()
			c.MockGet = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
				if diff := cmp.Diff(client.ObjectKey{Name: string(uid), Namespace: tc.wantNS}, key); diff != "" {
					t.Errorf("%s\nUntrack lookup: -want, +got:\n%s", tc.reason, diff)
				}
				obj.SetFinalizers([]string{ProviderConfigUsageFinalizer})
				return nil
			}
			c.MockUpdate = test.NewMockUpdateFn(nil, func(obj client.Object) error {
				if meta.FinalizerExists(obj.(ProviderConfigUsage), ProviderConfigUsageFinalizer) {
					t.Errorf("%s\nProviderConfigUsage finalizer was not removed", tc.reason)
				}
				return nil
			})

			mg := tc.mg()
			mg.SetUID(uid)
			mg.SetNamespace(namespace)
			if err := tc.cleaner(c).Untrack(context.Background(), mg); err != nil {
				t.Fatalf("%s\nUntrack returned an unexpected error: %v", tc.reason, err)
			}
		})
	}
}

// TestUntrackRejectsMismatchedScope ensures a tracker does not silently look for
// a usage in the wrong scope.
func TestUntrackRejectsMismatchedScope(t *testing.T) {
	uid := types.UID("some-uid")

	for name, tc := range map[string]struct {
		reason  string
		cleaner func(client.Client) ProviderConfigUsageCleaner
		mg      func() Managed
	}{
		"ModernTrackerGivenLegacyManaged": {
			reason: "The namespaced tracker cannot resolve a cluster-scoped managed resource's usage, so it must report an error.",
			cleaner: func(c client.Client) ProviderConfigUsageCleaner {
				return NewProviderConfigUsageTracker(c, &fake.ProviderConfigUsage{})
			},
			mg: func() Managed { return &fake.LegacyManaged{} },
		},
		"LegacyTrackerGivenModernManaged": {
			reason: "The cluster-scoped tracker cannot resolve a namespaced managed resource's usage, so it must report an error.",
			cleaner: func(c client.Client) ProviderConfigUsageCleaner {
				return NewLegacyProviderConfigUsageTracker(c, &fake.LegacyProviderConfigUsage{})
			},
			mg: func() Managed { return &fake.ModernManaged{} },
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := test.NewMockClient()
			c.MockGet = func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
				t.Errorf("%s\nUntrack looked a ProviderConfigUsage up despite the scope mismatch", tc.reason)
				return nil
			}
			c.MockUpdate = test.NewMockUpdateFn(nil, func(_ client.Object) error {
				t.Errorf("%s\nUntrack updated a ProviderConfigUsage despite the scope mismatch", tc.reason)
				return nil
			})

			mg := tc.mg()
			mg.SetUID(uid)
			if err := tc.cleaner(c).Untrack(context.Background(), mg); err == nil {
				t.Errorf("%s\nUntrack(...): want an error, got nil", tc.reason)
			}
		})
	}
}
