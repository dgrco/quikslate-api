package service

import (
	"context"
	"errors"
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
		{"admin can act on employee", domain.Admin, domain.Employee, true},
		{"admin can act on manager", domain.Admin, domain.Manager, true},
		{"admin can act on location lead", domain.Admin, domain.LocationLead, true},
		{"admin can act on another admin", domain.Admin, domain.Admin, true}, // NOTE: is this problematic?

		{"location lead can act on employee", domain.LocationLead, domain.Employee, true},
		{"location lead can act on manager", domain.LocationLead, domain.Manager, true},
		{"location lead cannot act on another location lead", domain.LocationLead, domain.LocationLead, false},
		{"location lead cannot act on admin", domain.LocationLead, domain.Admin, false},

		{"manager can act on employee", domain.Manager, domain.Employee, true},
		{"manager cannot act on another manager", domain.Manager, domain.Manager, false},
		{"manager cannot act on location lead", domain.Manager, domain.LocationLead, false},

		{"employee cannot act on another employee", domain.Employee, domain.Employee, false},
		{"non-admin with no role at this location cannot act on anyone", domain.EmptyRole, domain.Employee, false},
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

// fakeLocationRepo is a minimal domain.Repo test double. domain.Repo is a big
// interface (it embeds every sub-repo), but getAndValidateLocation only ever
// calls GetLocationById — so we embed the (nil) interface to satisfy the
// domain.Repo parameter type, and override just the one method we need.
// Calling any other method on it would nil-panic, which is fine: this test
// never exercises them.
type fakeLocationRepo struct {
	domain.Repo
	location domain.Location
}

func (f *fakeLocationRepo) GetLocationById(ctx context.Context, locationId string) (domain.Location, error) {
	return f.location, nil
}

// locationCtx builds the context RequireLocationMember would have populated
// for a location-scoped request.
func locationCtx(businessId, locationId string, isAdmin bool) context.Context {
	ctx := context.WithValue(context.Background(), ctxkeys.BusinessId, businessId)
	ctx = context.WithValue(ctx, ctxkeys.LocationId, locationId)
	ctx = context.WithValue(ctx, ctxkeys.IsAdmin, isAdmin)
	return ctx
}

func TestGetAndValidateLocation(t *testing.T) {
	cases := []struct {
		name      string
		callerCtx context.Context
		location  domain.Location
		wantErr   error
	}{
		{
			// Regression case: RequireLocationMember's authz query now also
			// enforces this at the source (see the join on locations in
			// GetLocationMemberAuthzContext's SQL, internal/infra/repo/
			// authz.go), but this service-layer check remains as the second
			// line of defense — and it's the only check for callers that
			// reach getAndValidateLocation via a resource lookup rather than
			// the route's own {locationId} (e.g. getAndValidateShift checking
			// a shift's stored locationId), which the middleware can't see.
			name:      "admin cannot act on a location belonging to a different business",
			callerCtx: locationCtx("business-A", "location-in-B", true),
			location:  domain.Location{Id: "location-in-B", BusinessId: "business-B"},
			wantErr:   domain.ErrForbidden,
		},
		{
			name:      "admin can act on any location within their own business, even outside their session location",
			callerCtx: locationCtx("business-A", "session-location", true),
			location:  domain.Location{Id: "other-location", BusinessId: "business-A"},
			wantErr:   nil,
		},
		{
			name:      "non-admin cannot act on a location belonging to a different business",
			callerCtx: locationCtx("business-A", "location-1", false),
			location:  domain.Location{Id: "location-1", BusinessId: "business-B"},
			wantErr:   domain.ErrForbidden,
		},
		{
			name:      "non-admin cannot act on a different location within their own business",
			callerCtx: locationCtx("business-A", "session-location", false),
			location:  domain.Location{Id: "other-location", BusinessId: "business-A"},
			wantErr:   domain.ErrForbidden,
		},
		{
			name:      "non-admin can act on their own session location",
			callerCtx: locationCtx("business-A", "location-1", false),
			location:  domain.Location{Id: "location-1", BusinessId: "business-A"},
			wantErr:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeLocationRepo{location: tc.location}
			_, err := getAndValidateLocation(tc.callerCtx, repo, tc.location.Id)
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("getAndValidateLocation() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
