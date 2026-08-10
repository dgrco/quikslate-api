package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// positions_repo.go implements domain.PositionRepository: CRUD for
// positions, the job roles (e.g. "cashier") that shifts and employee
// qualifications reference.

const (
	createPositionQuery = `
		INSERT INTO positions (business_id, name)
		VALUES ($1, $2)
		RETURNING id, business_id, name, created_at, updated_at
	`
	getPositionByIdQuery = `
		SELECT id, business_id, name, created_at, updated_at
		FROM positions
		WHERE id = $1
	`
	getPositionsByBusinessIdQuery = `
		SELECT id, business_id, name, created_at, updated_at
		FROM positions
		WHERE business_id = $1
	`
	changePositionNameQuery = `
		UPDATE positions
		SET name = $1, updated_at = NOW()
		WHERE id = $2
	`
	deletePositionQuery = `
		DELETE FROM positions
		WHERE id = $1
	`
)

// scanPositionFields scans a row's positions columns into p using scan
// (either row.Scan or rows.Scan).
func scanPositionFields(p *domain.Position, scan func(...any) error) error {
	return scan(
		&p.Id,
		&p.BusinessId,
		&p.Name,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
}

// scanPosition scans a single row into a domain.Position.
func scanPosition(row pgx.Row) (domain.Position, error) {
	var p domain.Position
	err := scanPositionFields(&p, row.Scan)
	if err != nil {
		return domain.Position{}, err
	}
	return p, nil
}

// scanPositions scans every remaining row into a slice of domain.Position.
func scanPositions(rows pgx.Rows) ([]domain.Position, error) {
	positions := []domain.Position{}
	for rows.Next() {
		var p domain.Position
		err := scanPositionFields(&p, rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position: %w", err)
		}
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate positions: %w", err)
	}
	return positions, nil
}

// CreatePosition inserts a new position under businessId, returning
// domain.ErrAlreadyExists if that business already has a position with this
// name.
func (r *PgRepository) CreatePosition(ctx context.Context, businessId, name string) (domain.Position, error) {
	p, err := scanPosition(r.exec.QueryRow(ctx, createPositionQuery, businessId, name))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.Position{}, domain.ErrAlreadyExists
		}
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}
	return p, nil
}

// GetPositionById fetches a position by id, returning domain.ErrNotFound if
// no such position exists. Callers that need to enforce it belongs to a
// particular business must check the returned BusinessId themselves; this
// does no business-scoping on its own.
func (r *PgRepository) GetPositionById(ctx context.Context, id string) (domain.Position, error) {
	p, err := scanPosition(r.exec.QueryRow(ctx, getPositionByIdQuery, id))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Position{}, domain.ErrNotFound
		default:
			return domain.Position{}, fmt.Errorf("failed to get position by ID: %w", err)
		}
	}
	return p, nil
}

// GetPositionsByBusinessId returns every position under businessId.
func (r *PgRepository) GetPositionsByBusinessId(ctx context.Context, businessId string) ([]domain.Position, error) {
	rows, err := r.exec.Query(ctx, getPositionsByBusinessIdQuery, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions by business ID: %w", err)
	}
	defer rows.Close()

	positions, err := scanPositions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions by business ID: %w", err)
	}
	return positions, nil
}

// ChangePositionName renames a position, returning domain.ErrNotFound if id
// doesn't match any row, or domain.ErrAlreadyExists if the new name collides
// with another position in the same business.
func (r *PgRepository) ChangePositionName(ctx context.Context, id, name string) error {
	cmdTag, err := r.exec.Exec(ctx, changePositionNameQuery, name, id)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("failed to change position name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeletePosition deletes a position by id, returning domain.ErrNotFound if
// id doesn't match any row.
func (r *PgRepository) DeletePosition(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, deletePositionQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.PositionRepository = (*PgRepository)(nil)
