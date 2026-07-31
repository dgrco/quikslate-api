package service

import (
	"context"
	"testing"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

func TestCanActOnRole(t *testing.T) {
	cases := []struct {
		name       string
		callerRole domain.LRole
		targetRole domain.LRole
		want       bool
	}{
		{"admin can act on employee", domain.EmptyRole, domain.Employee, true},
		{"admin can act on manager", domain.EmptyRole, domain.Manager, true},
		{"admin can act on location lead", domain.EmptyRole, domain.LocationLead, true},
		{"admin can act on another admin", domain.EmptyRole, domain.EmptyRole, true}, // NOTE: is this problematic?

		{"location lead can act on employee", domain.LocationLead, domain.Employee, true},
		{"location lead can act on manager", domain.LocationLead, domain.Manager, true},
		{"location lead cannot act on another location lead", domain.LocationLead, domain.LocationLead, false},
		{"location lead cannot act on admin", domain.LocationLead, domain.EmptyRole, false},

		{"manager can act on employee", domain.Manager, domain.Employee, true},
		{"manager cannot act on another manager", domain.Manager, domain.Manager, false},
		{"manager cannot act on location lead", domain.Manager, domain.LocationLead, false},

		{"employee cannot act on another employee", domain.Employee, domain.Employee, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canActOnRole(tc.callerRole, tc.targetRole)
			if got != tc.want {
				t.Errorf("canActOnRole(%q, %q) = %v, want %v", tc.callerRole, tc.targetRole, got, tc.want)
			}
		})
	}
}

func ctxWithAdminFlags(isAdmin, isPrimaryAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), ctxkeys.IsAdmin, isAdmin)
	ctx = context.WithValue(ctx, ctxkeys.IsPrimaryAdmin, isPrimaryAdmin)
	return ctx
}

func TestCanActOnBusinessMember(t *testing.T) {
	cases := []struct {
		name            string
		callerIsAdmin   bool
		callerIsPrimary bool
		target          *domain.BusinessMember
		want            bool
	}{
		{
			name:            "primary admin can act on a plain member",
			callerIsAdmin:   true,
			callerIsPrimary: true,
			target:          &domain.BusinessMember{IsAdmin: false, IsPrimaryAdmin: false},
			want:            true,
		},
		{
			name:            "primary admin can act on a regular admin",
			callerIsAdmin:   true,
			callerIsPrimary: true,
			target:          &domain.BusinessMember{IsAdmin: true, IsPrimaryAdmin: false},
			want:            true,
		},
		{
			name:            "primary admin cannot act on the primary admin",
			callerIsAdmin:   true,
			callerIsPrimary: true,
			target:          &domain.BusinessMember{IsAdmin: true, IsPrimaryAdmin: true},
			want:            false,
		},
		{
			name:            "regular admin can act on a plain member",
			callerIsAdmin:   true,
			callerIsPrimary: false,
			target:          &domain.BusinessMember{IsAdmin: false, IsPrimaryAdmin: false},
			want:            true,
		},
		{
			name:            "regular admin cannot act on another admin",
			callerIsAdmin:   true,
			callerIsPrimary: false,
			target:          &domain.BusinessMember{IsAdmin: true, IsPrimaryAdmin: false},
			want:            false,
		},
		{
			name:            "regular admin cannot act on the primary admin",
			callerIsAdmin:   true,
			callerIsPrimary: false,
			// NOTE: edge case: primary admins should never NOT be an admin as well
			target: &domain.BusinessMember{IsAdmin: false, IsPrimaryAdmin: true},
			want:   false,
		},
		{
			name:            "non-admin cannot act on anyone",
			callerIsAdmin:   false,
			callerIsPrimary: false,
			target:          &domain.BusinessMember{IsAdmin: false, IsPrimaryAdmin: false},
			want:            false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ctxWithAdminFlags(tc.callerIsAdmin, tc.callerIsPrimary)
			got := canActOnBusinessMember(ctx, tc.target)
			if got != tc.want {
				t.Errorf("canActOnBusinessMember() = %v, want %v", got, tc.want)
			}
		})
	}
}
