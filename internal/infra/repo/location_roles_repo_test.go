package repo

import (
	"errors"
	"testing"

	"github.com/dgrco/quikslate/internal/domain"
)

func TestAssignRoleRejectsLocationFromDifferentBusiness(t *testing.T) {
	txRepo := startRollbackTransaction(t)
	businessA := addBusiness(t, txRepo)
	locationA := addLocation(t, txRepo, businessA.Id)
	businessB := addBusiness(t, txRepo)
	employeeB := addUser(t, txRepo, "Employee B")

	err := txRepo.AssignRole(t.Context(), employeeB.Id, businessB.Id, locationA.Id, domain.LocationLead)
	if err == nil {
		t.Error("expected error, got success")
	} else {
		expectedErr := domain.ErrNotFound
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error (%v), got (%v)", expectedErr, err)
		}
	}
}

func TestAssignRolePreventsOverwritingRoleAtDifferentBusiness(t *testing.T) {
	txRepo := startRollbackTransaction(t)
	businessA := addBusiness(t, txRepo)
	businessB := addBusiness(t, txRepo)
	locationB := addLocation(t, txRepo, businessB.Id)
	employeeAB := addUser(t, txRepo, "Employee A/B")

	err := txRepo.AssignRole(t.Context(), employeeAB.Id, businessB.Id, locationB.Id, domain.Employee)
	if err != nil {
		t.Fatalf("assigning role failed: %v", err)
	}

	// Should fail since we are assigning using businessA and locationB (from businessB)
	err = txRepo.AssignRole(t.Context(), employeeAB.Id, businessA.Id, locationB.Id, domain.LocationLead)
	if err == nil {
		t.Error("expected error, got success")
	} else {
		expectedErr := domain.ErrNotFound
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error (%v), got (%v)", expectedErr, err)
		}
	}

	// Upsert should fail since locationB does not belong to businessA
	locationRole, err := txRepo.GetLocationRole(t.Context(), employeeAB.Id, locationB.Id, businessB.Id)
	if err != nil {
		t.Fatalf("failed to get location role: %v", err)
	}
	if locationRole.Role != domain.Employee {
		t.Error("expected no update to location role, got upserted on invalid location-business combination")
	}
}
