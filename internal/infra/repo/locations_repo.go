package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	createLocationQuery = `
		INSERT INTO locations (business_id, name, address)
		VALUES ($1, $2, $3)
		RETURNING id, business_id, name, address, created_at, updated_at
	`
	getLocationByIdQuery = `
		SELECT id, business_id, name, address, created_at, updated_at
		FROM locations
		WHERE id = $1
	`
	getLocationsByBusinessIdQuery = `
		SELECT id, business_id, name, address, created_at, updated_at
		FROM locations
		WHERE business_id = $1
	`
	deleteLocationQuery = `
		DELETE FROM locations
		WHERE id = $1
	`
)

func scanLocationFields(l *domain.Location, scan func(...any) error) error {
	return scan(
		&l.Id,
		&l.BusinessId,
		&l.Name,
		&l.Address,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
}

func scanLocation(row pgx.Row) (domain.Location, error) {
	var l domain.Location
	err := scanLocationFields(&l, row.Scan)
	if err != nil {
		return domain.Location{}, err
	}
	return l, nil
}

func scanLocations(rows pgx.Rows) ([]domain.Location, error) {
	locations := []domain.Location{}
	for rows.Next() {
		var l domain.Location
		err := scanLocationFields(&l, rows.Scan)
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

func (r *PgRepository) CreateLocation(ctx context.Context, businessId, name string, address *string) (domain.Location, error) {
	l, err := scanLocation(r.exec.QueryRow(ctx, createLocationQuery, businessId, name, address))
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}
	return l, nil
}

func (r *PgRepository) GetLocationById(ctx context.Context, id string) (domain.Location, error) {
	l, err := scanLocation(r.exec.QueryRow(ctx, getLocationByIdQuery, id))
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

func (r *PgRepository) GetLocationsByBusinessId(ctx context.Context, businessId string) ([]domain.Location, error) {
	rows, err := r.exec.Query(ctx, getLocationsByBusinessIdQuery, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations by business ID: %w", err)
	}
	defer rows.Close()

	locations, err := scanLocations(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations by business ID: %w", err)
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
		return nil // nothing changed
	}

	query, args := builder.Build("locations", "id", id)

	cmdTag, err := r.exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *PgRepository) DeleteLocation(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteLocationQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

var _ domain.LocationRepository = (*PgRepository)(nil)
