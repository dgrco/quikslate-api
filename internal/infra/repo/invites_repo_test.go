package repo

import (
	"errors"
	"testing"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
)

const token_hash = "abc123"
const email = "test@test.com"

func TestCreateInviteRejectsLocationFromDifferentBusiness(t *testing.T) {
	txRepo := startRollbackTransaction(t)
	businessA := addBusiness(t, txRepo)
	locationA := addLocation(t, txRepo, businessA.Id)
	businessB := addBusiness(t, txRepo)

	_, err := txRepo.CreateInvite(
		t.Context(),
		token_hash,
		email,
		businessB.Id,
		locationA.Id,
		domain.Employee,
		time.Now().Add(15*time.Minute),
	)
	if err == nil {
		t.Fatal("expected error, got success")
	} else {
		expectedErr := domain.ErrNotFound
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error (%v), got (%v)", expectedErr, err)
		}
	}
}
