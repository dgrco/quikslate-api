package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PgRepository) CreatePosition(ctx context.Context, businessID, name string) (domain.Position, error) {
	query := `
		INSERT INTO positions (business_id, name)
		VALUES ($1, $2)
		RETURNING id, business_id, name, created_at, updated_at
	`

	var p domain.Position
	err := r.pool.QueryRow(ctx, query, businessID, name).Scan(
		&p.ID,
		&p.BusinessID,
		&p.Name,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}

	return p, nil
}

func (r *PgRepository) GetPositionByID(ctx context.Context, id string) (domain.Position, error) {
	query := `
		SELECT id, business_id, name, created_at, updated_at
		FROM positions
		WHERE id = $1
	`

	var p domain.Position
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.BusinessID,
		&p.Name,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
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

func (r *PgRepository) GetPositionsByBusinessID(ctx context.Context, businessID string) ([]domain.Position, error) {
	query := `
		SELECT id, business_id, name, created_at, updated_at
		FROM positions
		WHERE business_id = $1
	`

	rows, err := r.pool.Query(ctx, query, businessID)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions by business ID: %w", err)
	}
	defer rows.Close()

	var positions []domain.Position
	for rows.Next() {
		var p domain.Position
		err := rows.Scan(
			&p.ID,
			&p.BusinessID,
			&p.Name,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
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

func (r *PgRepository) ChangePositionName(ctx context.Context, id, name string) error {
	query := `
		UPDATE positions
		SET name = $1, updated_at = NOW()
		WHERE id = $2
	`

	cmdTag, err := r.pool.Exec(ctx, query, name, id)
	if err != nil {
		return fmt.Errorf("failed to change position name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PgRepository) DeletePosition(ctx context.Context, id string) error {
	query := `
		DELETE FROM positions
		WHERE id = $1
	`

	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.PositionRepository = (*PgRepository)(nil)
