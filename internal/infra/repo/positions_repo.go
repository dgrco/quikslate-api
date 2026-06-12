package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

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

func scanPositionFields(p *domain.Position, scan func(...any) error) error {
	return scan(
		&p.Id,
		&p.BusinessId,
		&p.Name,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
}

func scanPosition(row pgx.Row) (domain.Position, error) {
	var p domain.Position
	err := scanPositionFields(&p, row.Scan)
	if err != nil {
		return domain.Position{}, err
	}
	return p, nil
}

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

func (r *PgRepository) CreatePosition(ctx context.Context, businessId, name string) (domain.Position, error) {
	p, err := scanPosition(r.pool.QueryRow(ctx, createPositionQuery, businessId, name))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.Position{}, domain.ErrAlreadyExists
		}
		return domain.Position{}, fmt.Errorf("failed to create position: %w", err)
	}
	return p, nil
}

func (r *PgRepository) GetPositionById(ctx context.Context, id string) (domain.Position, error) {
	p, err := scanPosition(r.pool.QueryRow(ctx, getPositionByIdQuery, id))
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

func (r *PgRepository) GetPositionsByBusinessId(ctx context.Context, businessId string) ([]domain.Position, error) {
	rows, err := r.pool.Query(ctx, getPositionsByBusinessIdQuery, businessId)
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

func (r *PgRepository) ChangePositionName(ctx context.Context, id, name string) error {
	cmdTag, err := r.pool.Exec(ctx, changePositionNameQuery, name, id)
	if err != nil {
		return fmt.Errorf("failed to change position name: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *PgRepository) DeletePosition(ctx context.Context, id string) error {
	cmdTag, err := r.pool.Exec(ctx, deletePositionQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete position: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.PositionRepository = (*PgRepository)(nil)
