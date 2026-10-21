package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
)

func TestOuterTransactionRollbackPreventsInnerTransactionCommit(t *testing.T) {
	rootTxRepo := startRollbackTransaction(t)

	outerTx, err := rootTxRepo.BeginTransaction(t.Context())
	if err != nil {
		t.Fatalf("failed to begin outer transaction: %v", err)
	}
	defer outerTx.Rollback(t.Context())

	outerTxRepo := rootTxRepo.WithTx(outerTx)

	innerTx, err := outerTxRepo.BeginTransaction(t.Context())
	if err != nil {
		t.Fatalf("failed to begin inner transaction: %v", err)
	}
	defer innerTx.Rollback(t.Context())

	innerTxRepo := outerTxRepo.WithTx(innerTx)
	b := addBusiness(t, innerTxRepo)

	if err := innerTx.Commit(t.Context()); err != nil {
		t.Fatalf("failed to commit inner transaction: %v", err)
	}

	if err := outerTx.Rollback(t.Context()); err != nil {
		t.Fatalf("failed to rollback outer transaction: %v", err)
	}

	_, err = rootTxRepo.GetBusinessById(t.Context(), b.Id)
	expectedErr := domain.ErrNotFound
	if err == nil {
		t.Errorf("expected error (%v), got success", expectedErr)
	} else if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected error (%v), got (%v)", expectedErr, err)
	}
}

func TestNestedTransactionsPreventDeadlock(t *testing.T) {
	// 3 seconds should be easily sufficient to indicate a deadlock
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(3*time.Second))

	u := addUser(t, pgRepo, "John Doe")
	t.Cleanup(func() {
		cancel()
		pgRepo.DeleteUser(context.Background(), u.Id)
	})

	outerTx, err := pgRepo.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("failed to begin outer transaction: %v", err)
	}
	defer outerTx.Rollback(ctx)

	outerTxRepo := pgRepo.WithTx(outerTx)

	// edit a row (obtains a lock)
	newName := "James Blake"
	if err := outerTxRepo.UpdateUser(ctx, u.Id, domain.UserUpdate{Name: &newName}); err != nil {
		t.Fatalf("failed to edit user's name in outer transaction: %v", err)
	}

	innerTx, err := outerTxRepo.BeginTransaction(ctx)
	if err != nil {
		t.Fatalf("failed to begin inner transaction: %v", err)
	}
	defer innerTx.Rollback(ctx)

	innerTxRepo := outerTxRepo.WithTx(innerTx)

	// Try to edit the user above (deadlock!)
	newName = "Jane Smith"
	err = innerTxRepo.UpdateUser(ctx, u.Id, domain.UserUpdate{Name: &newName})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("context deadline exceeded: this likely indicates a deadlock")
		} else {
			t.Fatalf("failed to edit user's name in inner transaction: %v", err)
		}
	}

	if err := innerTx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit inner transaction: %v", err)
	}
}
