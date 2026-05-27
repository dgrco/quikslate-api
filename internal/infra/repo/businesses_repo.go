package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/autoflow/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PgRepository) CreateBusiness(ctx context.Context, name string) (domain.Business, error) {
	query := `
		INSERT INTO businesses (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`

	var b domain.Business
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&b.ID,
		&b.Name,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if err != nil {
		return domain.Business{}, fmt.Errorf("failed to create business: %w", err)
	}

	return b, nil
}

func (r *PgRepository) GetBusinessById(ctx context.Context, id string) (domain.Business, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM businesses
		WHERE id = $1
	`

	var b domain.Business
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.Name,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Business{}, domain.ErrNotFound
		default:
			return domain.Business{}, fmt.Errorf("failed to get business by ID: %w", err)
		}
	}

	return b, nil
}

func (r *PgRepository) ChangeBusinessName(ctx context.Context, id, newName string) error {
	query := `
		UPDATE businesses
		SET name = $1, updated_at = NOW()
		WHERE id = $2
	`

	cmdTag, err := r.pool.Exec(ctx, query, newName, id)
	if err != nil {
		return fmt.Errorf("failed to change business name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *PgRepository) DeleteBusiness(ctx context.Context, id string) error {
	query := `
		DELETE FROM businesses
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}

	return nil
}

var _ domain.BusinessRepository = (*PgRepository)(nil)
