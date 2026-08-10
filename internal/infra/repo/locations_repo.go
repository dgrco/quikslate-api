package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

// locations_repo.go implements domain.LocationRepository: CRUD for
// locations, which belong to a business and in turn own positions, shifts,
// and location roles.

const (
	createLocationQuery = `
		INSERT INTO locations (business_id, name, address, timezone)
		VALUES ($1, $2, $3, $4)
		RETURNING id, business_id, name, address, timezone, created_at, updated_at
	`
	getLocationByIdQuery = `
		SELECT id, business_id, name, address, timezone, created_at, updated_at
		FROM locations
		WHERE id = $1
	`
	getLocationsByBusinessIdQuery = `
		SELECT id, business_id, name, address, timezone, created_at, updated_at
		FROM locations
		WHERE business_id = $1
		ORDER BY created_at
	`
	updateLocationQuery = `
		UPDATE locations
		SET name = $1, address = $2, timezone = $3, updated_at = NOW()
		WHERE id = $4
	`
	deleteLocationQuery = `
		DELETE FROM locations
		WHERE id = $1
	`
)

// scanLocationFields scans a row's locations columns into l using scan
// (either row.Scan or rows.Scan).
func scanLocationFields(l *domain.Location, scan func(...any) error) error {
	return scan(
		&l.Id,
		&l.BusinessId,
		&l.Name,
		&l.Address,
		&l.Timezone,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
}

// scanLocation scans a single row into a domain.Location.
func scanLocation(row pgx.Row) (domain.Location, error) {
	var l domain.Location
	err := scanLocationFields(&l, row.Scan)
	if err != nil {
		return domain.Location{}, err
	}
	return l, nil
}

// scanLocations scans every remaining row into a slice of domain.Location.
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

// CreateLocation inserts a new location under businessId and returns it.
func (r *PgRepository) CreateLocation(ctx context.Context, businessId, name string, address *string, timezone string) (domain.Location, error) {
	l, err := scanLocation(r.exec.QueryRow(ctx, createLocationQuery, businessId, name, address, timezone))
	if err != nil {
		return domain.Location{}, fmt.Errorf("failed to create location: %w", err)
	}
	return l, nil
}

// GetLocationById fetches a location by id, returning domain.ErrNotFound if
// no such location exists. Callers that need to enforce it belongs to a
// particular business must check the returned BusinessId themselves; this
// does no business-scoping on its own.
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

// GetLocationsByBusinessId returns every location under businessId,
// oldest-created first.
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

// UpdateLocationById overwrites a location's name, address, and timezone,
// returning domain.ErrNotFound if id doesn't match any row.
func (r *PgRepository) UpdateLocationById(ctx context.Context, id string, update domain.LocationUpdate) error {
	cmdTag, err := r.exec.Exec(ctx, updateLocationQuery, update.Name, update.Address, update.Timezone, id)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// DeleteLocation deletes a location by id, returning domain.ErrNotFound if
// id doesn't match any row. Positions, shifts, location_roles, etc. cascade
// via FK ON DELETE CASCADE.
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
