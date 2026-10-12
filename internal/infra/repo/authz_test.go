package repo

import (
	"errors"
	"testing"

	"github.com/dgrco/quikslate/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestGetBusinessMemberAuthzContext(t *testing.T) {
	cases := []struct {
		name           string
		isPrimaryAdmin bool
		isAdmin        bool
	}{
		{"admin but not primary admin", false, true},
		{"primary admin", true, true},
		{"standard member", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			txRepo := startRollbackTransaction(t)
			b := addBusiness(t, txRepo)
			u := addUser(t, txRepo, "John Doe")
			if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b.Id, tc.isPrimaryAdmin, tc.isAdmin); err != nil {
				t.Fatalf("failed to add user to business: %v", err)
			}
			authzResult, err := txRepo.GetBusinessMemberAuthzContext(t.Context(), u.Id, b.Id)
			if err != nil {
				t.Fatalf("failed to get business member authz result: %v", err)
			}
			if authzResult.IsPrimaryAdmin != tc.isPrimaryAdmin || authzResult.IsAdmin != tc.isAdmin {
				t.Errorf(
					"expected is_primary_admin=%v and is_admin=%v, got is_primary_admin=%v and is_admin=%v",
					tc.isPrimaryAdmin,
					tc.isAdmin,
					authzResult.IsPrimaryAdmin,
					authzResult.IsAdmin,
				)
			}
		})
	}
}

func TestGetLocationMemberAuthzContext(t *testing.T) {
	cases := []struct {
		name           string
		isPrimaryAdmin bool
		isAdmin        bool
		assignedRole   domain.LRole
	}{
		{"admin but not primary admin", false, true, domain.EmptyRole},
		{"primary admin", true, true, domain.EmptyRole},
		{"standard member", false, false, domain.Employee},
		{"non-admin with empty role", false, false, domain.EmptyRole},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			txRepo := startRollbackTransaction(t)
			b := addBusiness(t, txRepo)
			l := addLocation(t, txRepo, b.Id)
			u := addUser(t, txRepo, "John Doe")
			if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b.Id, tc.isPrimaryAdmin, tc.isAdmin); err != nil {
				t.Fatalf("failed to add user to business: %v", err)
			}

			if tc.assignedRole != domain.EmptyRole {
				if err := txRepo.AssignRole(t.Context(), u.Id, b.Id, l.Id, tc.assignedRole); err != nil {
					t.Fatalf("failed to add location role: %v", err)
				}
			}

			authzResult, err := txRepo.GetLocationMemberAuthzContext(t.Context(), u.Id, b.Id, l.Id)
			if err != nil {
				t.Fatalf("failed to get location member authz context: %v", err)
			}
			if authzResult.IsPrimaryAdmin != tc.isPrimaryAdmin || authzResult.IsAdmin != tc.isAdmin {
				t.Errorf(
					"expected is_primary_admin=%v and is_admin=%v, got is_primary_admin=%v and is_admin=%v",
					tc.isPrimaryAdmin,
					tc.isAdmin,
					authzResult.IsPrimaryAdmin,
					authzResult.IsAdmin,
				)
			}
			if authzResult.Role != tc.assignedRole {
				t.Errorf("expected role=%q, got %q", tc.assignedRole, authzResult.Role)
			}
		})
	}
}

func TestGetLocationMemberAuthzContextFailsOnLocationFromAnotherBusiness(t *testing.T) {
	txRepo := startRollbackTransaction(t)
	b1 := addBusiness(t, txRepo)
	b2 := addBusiness(t, txRepo)
	l2 := addLocation(t, txRepo, b2.Id)

	u := addUser(t, txRepo, "John Doe")
	if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b1.Id, false, false); err != nil {
		t.Fatalf("failed to add user to business: %v", err)
	}

	_, err := txRepo.GetLocationMemberAuthzContext(t.Context(), u.Id, b1.Id, l2.Id)
	expectedErr := domain.ErrForbidden
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error (%v), got (%v)", expectedErr, err)
	}
}
