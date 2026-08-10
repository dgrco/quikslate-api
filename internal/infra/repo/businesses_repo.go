package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

// businesses_repo.go implements domain.BusinessRepository: CRUD for the
// businesses table, the top-level tenant every location, position, and
// shift belongs to.

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
	getBusinessesByUserIdQuery = `
		SELECT b.id, b.name, b.created_at, b.updated_at
		FROM businesses b
		JOIN business_members bm ON bm.business_id = b.id
		WHERE bm.user_id = $1
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

// scanBusiness scans a single row into a domain.Business.
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

// CreateBusiness inserts a new business row and returns it.
func (r *PgRepository) CreateBusiness(ctx context.Context, name string) (domain.Business, error) {
	b, err := scanBusiness(r.exec.QueryRow(ctx, createBusinessQuery, name))
	if err != nil {
		return domain.Business{}, fmt.Errorf("failed to create business: %w", err)
	}
	return b, nil
}

// GetBusinessById fetches a business by id, returning domain.ErrNotFound if
// no such business exists.
func (r *PgRepository) GetBusinessById(ctx context.Context, id string) (domain.Business, error) {
	b, err := scanBusiness(r.exec.QueryRow(ctx, getBusinessByIdQuery, id))
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

// GetBusinessesByUserId returns every business userId is a member of, via a
// join through business_members. Returns an empty slice, not an error, if
// they're a member of none.
func (r *PgRepository) GetBusinessesByUserId(ctx context.Context, userId string) ([]domain.Business, error) {
	rows, err := r.exec.Query(ctx, getBusinessesByUserIdQuery, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get businesses by user ID: %w", err)
	}
	defer rows.Close()

	businesses := []domain.Business{}
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(&b.Id, &b.Name, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan business: %w", err)
		}
		businesses = append(businesses, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate businesses: %w", err)
	}
	return businesses, nil
}

// ChangeBusinessName renames a business, returning domain.ErrNotFound if id
// doesn't match any row.
func (r *PgRepository) ChangeBusinessName(ctx context.Context, id, newName string) error {
	cmdTag, err := r.exec.Exec(ctx, changeBusinessNameQuery, newName, id)
	if err != nil {
		return fmt.Errorf("failed to change business name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteBusiness deletes a business by id, returning domain.ErrNotFound if
// id doesn't match any row. Locations, members, etc. cascade via FK
// ON DELETE CASCADE.
func (r *PgRepository) DeleteBusiness(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteBusinessQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete business: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.BusinessRepository = (*PgRepository)(nil)
