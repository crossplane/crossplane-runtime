/*
Copyright 2023 The Crossplane Authors.

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

package managed

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
)

func TestTestManagementPoliciesResolverIsPaused(t *testing.T) {
	type args struct {
		enabled bool
		policy  xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"Disabled": {
			reason: "Should return false if management policies are disabled",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{},
			},
			want: false,
		},
		"EnabledEmptyPolicies": {
			reason: "Should return true if the management policies are enabled and empty",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{},
			},
			want: true,
		},
		"EnabledNonEmptyPolicies": {
			reason: "Should return true if the management policies are enabled and non empty",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.enabled, tc.args.policy, xpv1.DeletionDelete)
			if diff := cmp.Diff(tc.want, r.IsPaused()); diff != "" {
				t.Errorf("\nReason: %s\nIsPaused(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagementPoliciesResolverValidate(t *testing.T) {
	type args struct {
		enabled bool
		policy  xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Enabled": {
			reason: "Should return nil if the management policy is enabled.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{},
			},
			want: nil,
		},
		"DisabledNonDefault": {
			reason: "Should return error if the management policy is non-default and disabled.",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionCreate},
			},
			want: cmpopts.AnyError,
		},
		"DisabledDefault": {
			reason: "Should return nil if the management policy is default and disabled.",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: nil,
		},
		"EnabledSupported": {
			reason: "Should return nil if the management policy is supported.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: nil,
		},
		"EnabledNotSupported": {
			reason: "Should return err if the management policy is not supported.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
			},
			want: cmpopts.AnyError,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.enabled, tc.args.policy, xpv1.DeletionDelete)
			if diff := cmp.Diff(tc.want, r.Validate(), cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nIsNonDefault(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagementPoliciesResolverShouldCreate(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasCreate": {
			reason: "Should return true if management policies are enabled and managementPolicies has action Create",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionCreate},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasCreateAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have Create",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldCreate()); diff != "" {
				t.Errorf("\nReason: %s\nShouldCreate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagementPoliciesResolverShouldUpdate(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasUpdate": {
			reason: "Should return true if management policies are enabled and managementPolicies has action Update",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionUpdate},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasUpdateAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have Update",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldUpdate()); diff != "" {
				t.Errorf("\nReason: %s\nShouldUpdate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagementPoliciesResolverShouldLateInitialize(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasLateInitialize": {
			reason: "Should return true if management policies are enabled and managementPolicies has action LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionLateInitialize},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasLateInitializeAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldLateInitialize()); diff != "" {
				t.Errorf("\nReason: %s\nShouldLateInitialize(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagementPoliciesResolverOnlyObserve(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}
	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return false if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: false,
		},
		"ManagementPoliciesEnabledHasOnlyObserve": {
			reason: "Should return true if management policies are enabled and managementPolicies has action LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasMultipleActions": {
			reason: "Should return false if management policies are enabled and managementPolicies has multiple actions",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionLateInitialize, xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldOnlyObserve()); diff != "" {
				t.Errorf("\nReason: %s\nShouldOnlyObserve(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestShouldDelete(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		managed                   resource.Managed
	}
	type want struct {
		delete bool
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DeletionOrphan": {
			reason: "Should orphan if management policies are disabled and deletion policy is set to Orphan.",
			args: args{
				managementPoliciesEnabled: false,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
				},
			},
			want: want{delete: false},
		},
		"DeletionDelete": {
			reason: "Should delete if management policies are disabled and deletion policy is set to Delete.",
			args: args{
				managementPoliciesEnabled: false,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionDeleteManagementActionAll": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Delete and management policy is set to All.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionAll},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionOrphanManagementActionAll": {
			reason: "Should orphan if management policies are enabled and deletion policy is set to Orphan and management policy is set to All.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionAll},
					},
				},
			},
			want: want{delete: false},
		},
		"DeletionDeleteManagementActionDelete": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Delete and management policy has action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionOrphanManagementActionDelete": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Orphan and management policy has action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionDeleteManagementActionNoDelete": {
			reason: "Should orphan if management policies are enabled and deletion policy is set to Delete and management policy does not have action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.Managed{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
					},
				},
			},
			want: want{delete: false},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.managed.GetManagementPolicies(), tc.args.managed.GetDeletionPolicy())
			if diff := cmp.Diff(tc.want.delete, r.ShouldDelete()); diff != "" {
				t.Errorf("\nReason: %s\nShouldDelete(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
