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

	"github.com/crossplaneio/crossplane-runtime/pkg/resource/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
)

var (
	_ ManagedConfiguratorFn = ConfigureNames
	_ ManagedConfiguratorFn = ConfigureReclaimPolicy
	_ ManagedConfigurator   = ConfiguratorChain{}
)

func TestConfiguratorChain(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		cm  Claim
		cs  Class
		mg  Managed
	}

	cases := map[string]struct {
		cc   ConfiguratorChain
		args args
		want error
	}{
		"EmptyChain": {
			cc: ConfiguratorChain{},
			args: args{
				ctx: context.Background(),
				cm:  &fake.MockClaim{},
				cs:  &fake.MockClass{},
				mg:  &fake.MockManaged{},
			},
			want: nil,
		},
		"SuccessulConfigurator": {
			cc: ConfiguratorChain{
				ManagedConfiguratorFn(func(_ context.Context, _ Claim, _ Class, _ Managed) error {
					return nil
				}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.MockClaim{},
				cs:  &fake.MockClass{},
				mg:  &fake.MockManaged{},
			},
			want: nil,
		},
		"ConfiguratorReturnsError": {
			cc: ConfiguratorChain{
				ManagedConfiguratorFn(func(_ context.Context, _ Claim, _ Class, _ Managed) error {
					return errBoom
				}),
			},
			args: args{
				ctx: context.Background(),
				cm:  &fake.MockClaim{},
				cs:  &fake.MockClass{},
				mg:  &fake.MockManaged{},
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cc.Configure(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.cc.Configure(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestNameConfigurators(t *testing.T) {
	claimName := "myclaim"
	claimNS := "myclaimns"
	externalName := "wayout"

	type args struct {
		ctx context.Context
		cm  Claim
		cs  Class
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				ctx: context.Background(),
				cm: &fake.MockClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   claimNS,
						Name:        claimName,
						Annotations: map[string]string{meta.ExternalNameAnnotationKey: externalName},
					}},
				mg: &fake.MockManaged{},
			},
			want: want{
				mg: &fake.MockManaged{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: claimNS + "-" + claimName + "-",
						Annotations:  map[string]string{meta.ExternalNameAnnotationKey: externalName},
					},
				},
			},
		},
	}

	t.Run("TestConfigureNames", func(t *testing.T) {
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				got := ConfigureNames(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
				if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
					t.Errorf("ConfigureNames(...): -want error, +got error:\n%s", diff)
				}

				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("ConfigureNames(...) Managed: -want, +got error:\n%s", diff)
				}
			})
		}
	})

	// NOTE(negz): This deprecated API wraps ConfigureNames; they should behave
	// identically.
	t.Run("TestObjectMetaConfigurator", func(t *testing.T) {
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				om := NewObjectMetaConfigurator(nil)
				got := om.Configure(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
				if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
					t.Errorf("om.Configure(...): -want error, +got error:\n%s", diff)
				}

				if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
					t.Errorf("om.Configure(...) Managed: -want, +got error:\n%s", diff)
				}
			})
		}
	})
}

func TestConfigureReclaimPolicy(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  Claim
		cs  Class
		mg  Managed
	}

	type want struct {
		err error
		mg  Managed
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AlreadySet": {
			reason: "Existing managed resource reclaim policies should be respected.",
			args: args{
				ctx: context.Background(),
				cs:  &fake.MockClass{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimDelete}},
				mg:  &fake.MockManaged{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimRetain}},
			},
			want: want{
				mg: &fake.MockManaged{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimRetain}},
			},
		},
		"SetByClass": {
			reason: "The class's reclaim policy should be propagated to the managed resource.",
			args: args{
				ctx: context.Background(),
				cs:  &fake.MockClass{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimRetain}},
				mg:  &fake.MockManaged{},
			},
			want: want{
				mg: &fake.MockManaged{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimRetain}},
			},
		},
		"DefaultToDelete": {
			reason: "If neither the class nor managed resource set a reclaim policy, it should default to Delete.",
			args: args{
				ctx: context.Background(),
				cs:  &fake.MockClass{},
				mg:  &fake.MockManaged{},
			},
			want: want{
				mg: &fake.MockManaged{MockReclaimer: fake.MockReclaimer{Policy: v1alpha1.ReclaimDelete}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConfigureReclaimPolicy(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, got, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nConfigureReclaimPolicy(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("\nReason: %s\nConfigureReclaimPolicy(...) Managed: -want, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
