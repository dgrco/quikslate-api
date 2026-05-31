package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	createBusinessQuery = `
		INSERT INTO businesses (name)
		VALUES ($1)
		RETURNING id, name, created_at, updated_at
	`
	getBusinessByIdQuery = `
		SELECT id, name, created_at, updated_at
		FROM businesses
		WHERE id = $1
	`
	changeBusinessNameQuery = `
		UPDATE businesses
		SET name = $1, updated_at = NOW()
		WHERE id = $2
	`
	deleteBusinessQuery = `
		DELETE FROM businesses
		WHERE id = $1
	`
)

func scanBusiness(row pgx.Row) (domain.Business, error) {
	var b domain.Business
	err := row.Scan(
		&b.Id,
		&b.Name,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		return domain.Business{}, err
	}
	return b, nil
}

func (r *PgRepository) CreateBusiness(ctx context.Context, name string) (domain.Business, error) {
	b, err := scanBusiness(r.pool.QueryRow(ctx, createBusinessQuery, name))
	if err != nil {
		return domain.Business{}, fmt.Errorf("failed to create business: %w", err)
	}
	return b, nil
}

func (r *PgRepository) GetBusinessById(ctx context.Context, id string) (domain.Business, error) {
	b, err := scanBusiness(r.pool.QueryRow(ctx, getBusinessByIdQuery, id))
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
	cmdTag, err := r.pool.Exec(ctx, changeBusinessNameQuery, newName, id)
	if err != nil {
		return fmt.Errorf("failed to change business name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PgRepository) DeleteBusiness(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, deleteBusinessQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}
	return nil
}

var _ domain.BusinessRepository = (*PgRepository)(nil)
