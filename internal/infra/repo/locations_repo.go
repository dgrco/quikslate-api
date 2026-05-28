package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PgRepository) CreateLocation(ctx context.Context, businessID, name, address string) (domain.Location, error) {
	query := `
		INSERT INTO locations (business_id, name, address)
		VALUES ($1, $2, $3)
		RETURNING id, business_id, name, address, created_at, updated_at
	`

	var l domain.Location
	err := r.pool.QueryRow(ctx, query, businessID, name, address).Scan(
		&l.ID,
		&l.BusinessID,
		&l.Name,
		&l.Address,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}

	return l, nil
}

func (r *PgRepository) GetLocationById(ctx context.Context, id string) (domain.Location, error) {
	query := `
		SELECT id, business_id, name, address, created_at, updated_at
		FROM locations
		WHERE id = $1
	`

	var l domain.Location
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&l.ID,
		&l.BusinessID,
		&l.Name,
		&l.Address,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Location{}, domain.ErrNotFound
		default:
			return domain.Location{}, fmt.Errorf("failed to get location by ID: %w", err)
		}
	}

	return l, nil
}

func (r *PgRepository) GetLocationsByBusinessID(ctx context.Context, businessID string) ([]domain.Location, error) {
	query := `
		SELECT id, business_id, name, address, created_at, updated_at
		FROM locations
		WHERE business_id = $1
	`

	var locations []domain.Location
	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations by business ID: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var l domain.Location
		err := rows.Scan(
			&l.ID,
			&l.BusinessID,
			&l.Name,
			&l.Address,
			&l.CreatedAt,
			&l.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations = append(locations, l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate locations: %w", err)
	}

	return locations, nil
}

func (r *PgRepository) UpdateLocationById(ctx context.Context, id string, update domain.LocationUpdate) error {
	builder := newUpdateBuilder()

	if update.Name != nil {
		builder.Add("name", *update.Name)
	}
	if update.Address != nil {
		builder.Add("address", *update.Address)
	}
	if builder.IsEmpty() {
		return nil
	} // nothing changed

	query, args := builder.Build("locations", "id", id)

	cmdTag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *PgRepository) DeleteLocation(ctx context.Context, id string) error {
	query := `
		DELETE FROM locations
		WHERE id = $1
	`
	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

var _ domain.LocationRepository = (*PgRepository)(nil)
