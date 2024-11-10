package managed

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

func TestManagementPoliciesResolver_Validate(t *testing.T) {
	type fields struct {
		enabled            bool
		managementPolicies sets.Set[xpv1.ManagementAction]
		deletionPolicy     xpv1.DeletionPolicy
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "DisabledWithDefaultPolicy",
			fields: fields{
				enabled:            false,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll),
			},
			wantErr: false,
		},
		{
			name: "DisabledWithNonDefaultPolicy",
			fields: fields{
				enabled:            false,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			wantErr: true,
		},
		{
			name: "EnabledWithSupportedPolicy",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			wantErr: false,
		},
		{
			name: "EnabledWithUnsupportedPolicy",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction]("UnsupportedAction"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagementPoliciesResolver(tt.fields.enabled, tt.fields.managementPolicies.UnsortedList(), tt.fields.deletionPolicy)
			if err := m.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("ManagementPoliciesResolver.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManagementPoliciesResolver_ShouldDelete(t *testing.T) {
	type fields struct {
		enabled            bool
		managementPolicies sets.Set[xpv1.ManagementAction]
		deletionPolicy     xpv1.DeletionPolicy
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "DisabledWithDeletionDelete",
			fields: fields{
				enabled:        false,
				deletionPolicy: xpv1.DeletionDelete,
			},
			want: true,
		},
		{
			name: "DisabledWithDeletionOrphan",
			fields: fields{
				enabled:        false,
				deletionPolicy: xpv1.DeletionOrphan,
			},
			want: false,
		},
		{
			name: "EnabledWithDeletePolicyAndAction",
			fields: fields{
				enabled:            true,
				deletionPolicy:     xpv1.DeletionDelete,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionDelete),
			},
			want: true,
		},
		{
			name: "EnabledWithDeleteActionOnly",
			fields: fields{
				enabled:            true,
				deletionPolicy:     xpv1.DeletionOrphan,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionDelete, xpv1.ManagementActionObserve),
			},
			want: true,
		},
		{
			name: "EnabledWithoutDeleteAction",
			fields: fields{
				enabled:            true,
				deletionPolicy:     xpv1.DeletionDelete,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagementPoliciesResolver(tt.fields.enabled, tt.fields.managementPolicies.UnsortedList(), tt.fields.deletionPolicy)
			if got := m.ShouldDelete(); got != tt.want {
				t.Errorf("ManagementPoliciesResolver.ShouldDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManagementPoliciesResolver_ShouldOnlyObserve(t *testing.T) {
	type fields struct {
		enabled            bool
		managementPolicies sets.Set[xpv1.ManagementAction]
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "Disabled",
			fields: fields{
				enabled: false,
			},
			want: false,
		},
		{
			name: "EnabledWithObserveOnly",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			want: true,
		},
		{
			name: "EnabledWithMultipleActions",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionCreate),
			},
			want: false,
		},
		{
			name: "EnabledWithObserveAndLateInitialize",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve, xpv1.ManagementActionLateInitialize),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagementPoliciesResolver(tt.fields.enabled, tt.fields.managementPolicies.UnsortedList(), xpv1.DeletionDelete)
			if got := m.ShouldOnlyObserve(); got != tt.want {
				t.Errorf("ManagementPoliciesResolver.ShouldOnlyObserve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManagementPoliciesResolver_ShouldUpdate(t *testing.T) {
	type fields struct {
		enabled            bool
		managementPolicies sets.Set[xpv1.ManagementAction]
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "Disabled",
			fields: fields{
				enabled: false,
			},
			want: true,
		},
		{
			name: "EnabledWithUpdateAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionUpdate),
			},
			want: true,
		},
		{
			name: "EnabledWithAllAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll),
			},
			want: true,
		},
		{
			name: "EnabledWithoutUpdateAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			want: false,
		},
		{
			name: "EnabledWithMultipleActionsIncludingUpdate",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionUpdate,
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionCreate,
				),
			},
			want: true,
		},
		{
			name: "EnabledWithObserveAndLateInitialize",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionLateInitialize,
				),
			},
			want: false,
		},
		{
			name: "EnabledWithObserveUpdateAndLateInitialize",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionUpdate,
					xpv1.ManagementActionLateInitialize,
				),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagementPoliciesResolver(tt.fields.enabled, tt.fields.managementPolicies.UnsortedList(), xpv1.DeletionDelete)
			if got := m.ShouldUpdate(); got != tt.want {
				t.Errorf("ManagementPoliciesResolver.ShouldUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManagementPoliciesResolver_ShouldLateInitialize(t *testing.T) {
	type fields struct {
		enabled            bool
		managementPolicies sets.Set[xpv1.ManagementAction]
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "Disabled",
			fields: fields{
				enabled: false,
			},
			want: true,
		},
		{
			name: "EnabledWithLateInitializeAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionLateInitialize),
			},
			want: true,
		},
		{
			name: "EnabledWithAllAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionAll),
			},
			want: true,
		},
		{
			name: "EnabledWithoutLateInitializeAction",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](xpv1.ManagementActionObserve),
			},
			want: false,
		},
		{
			name: "EnabledWithMultipleActionsIncludingLateInitialize",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionLateInitialize,
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionCreate,
				),
			},
			want: true,
		},
		{
			name: "EnabledWithEmptyPolicies",
			fields: fields{
				enabled:            true,
				managementPolicies: sets.New[xpv1.ManagementAction](),
			},
			want: false,
		},
		{
			name: "EnabledWithObserveAndLateInitialize",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionLateInitialize,
				),
			},
			want: true,
		},
		{
			name: "EnabledWithObserveUpdateAndLateInitialize",
			fields: fields{
				enabled: true,
				managementPolicies: sets.New[xpv1.ManagementAction](
					xpv1.ManagementActionObserve,
					xpv1.ManagementActionUpdate,
					xpv1.ManagementActionLateInitialize,
				),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagementPoliciesResolver(tt.fields.enabled, tt.fields.managementPolicies.UnsortedList(), xpv1.DeletionDelete)
			if got := m.ShouldLateInitialize(); got != tt.want {
				t.Errorf("ManagementPoliciesResolver.ShouldLateInitialize() = %v, want %v", got, tt.want)
			}
		})
	}
}
