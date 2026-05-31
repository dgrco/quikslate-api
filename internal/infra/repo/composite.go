package repo

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
)

// ===================================================================
// This file contains composite functions that write multiple tables.
// Therefore, all functions will utilize transactions, rather than a
// direct pgx.pool call.
// ===================================================================

func (r *PgRepository) CreateUserWithBusiness(
	ctx context.Context,
	email, password, businessName string,
) (domain.User, domain.Business, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, domain.Business{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	u, err := scanUser(tx.QueryRow(ctx, createUserQuery, email, password))
	if err != nil {
		return domain.User{}, domain.Business{}, fmt.Errorf("failed to create user: %w", err)
	}

	b, err := scanBusiness(tx.QueryRow(ctx, createBusinessQuery, businessName))
	if err != nil {
		return domain.User{}, domain.Business{}, fmt.Errorf("failed to create business: %w", err)
	}

	_, err = tx.Exec(ctx, assignRoleQuery, u.Id, b.Id, nil, domain.AdminRole)

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, domain.Business{}, fmt.Errorf("failed to commit: %w", err)
	}

	return u, b, nil
}

var _ domain.CompositeRepository = (*PgRepository)(nil)
