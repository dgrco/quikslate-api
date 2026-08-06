package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	createInviteQuery = `
		INSERT INTO invites (token_hash, email, business_id, location_id, role, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, token_hash, email, business_id, location_id, role, expires_at, accepted_at, created_at, updated_at
	`
	getInviteByIdQuery = `
		SELECT id, token_hash, email, business_id, location_id, role, expires_at, accepted_at, created_at, updated_at
		FROM invites
		WHERE id = $1
	`
	getInviteFromTokenHashQuery = `
		SELECT id, token_hash, email, business_id, location_id, role, expires_at, accepted_at, created_at, updated_at
		FROM invites
		WHERE token_hash = $1
	`
	getPendingInviteByEmailAndBusinessIdQuery = `
		SELECT id, token_hash, email, business_id, location_id, role, expires_at, accepted_at, created_at, updated_at
		FROM invites
		WHERE email = $1 AND business_id = $2 AND accepted_at IS NULL
	`
	markInviteAcceptedQuery = `
		UPDATE invites
		SET accepted_at = NOW(), updated_at = NOW()
		WHERE token_hash = $1
	`
	getPendingInvitesByLocationIdQuery = `
		SELECT id, token_hash, email, business_id, location_id, role, expires_at, accepted_at, created_at, updated_at
		FROM invites
		WHERE location_id = $1 AND accepted_at IS NULL
		ORDER BY created_at
	`
	deleteInviteQuery = `
		DELETE FROM invites
		WHERE id = $1
	`
)

// scanInviteFields scans a row's invites columns into inv using scan (either
// row.Scan or rows.Scan).
func scanInviteFields(inv *domain.Invite, scan func(...any) error) error {
	if err := scan(
		&inv.Id,
		&inv.TokenHash,
		&inv.Email,
		&inv.BusinessId,
		&inv.LocationId,
		&inv.Role,
		&inv.ExpiresAt,
		&inv.AcceptedAt,
		&inv.CreatedAt,
		&inv.UpdatedAt,
	); err != nil {
		return err
	}
	return nil
}

// scanInvite scans a single row into a domain.Invite.
func scanInvite(row pgx.Row) (domain.Invite, error) {
	var inv domain.Invite
	if err := scanInviteFields(&inv, row.Scan); err != nil {
		return domain.Invite{}, err
	}
	return inv, nil
}

// CreateInvite inserts a new invite for email to join businessId at
// locationId with role, hashed under tokenHash. Returns domain.ErrAlreadyExists
// if a unique-constraint violation occurs (e.g. a pending invite for this
// email/business already exists).
func (r *PgRepository) CreateInvite(
	ctx context.Context,
	tokenHash,
	email,
	businessId,
	locationId string,
	role domain.LRole,
	expiresAt time.Time,
) (domain.Invite, error) {
	inv, err := scanInvite(r.exec.QueryRow(ctx, createInviteQuery, tokenHash, email, businessId, locationId, role, expiresAt))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.Invite{}, domain.ErrAlreadyExists
		}
		return domain.Invite{}, fmt.Errorf("failed to create invite: %w", err)
	}
	return inv, nil
}

// GetInviteById fetches an invite by its id, returning domain.ErrNotFound if
// no such invite exists.
func (r *PgRepository) GetInviteById(ctx context.Context, id string) (domain.Invite, error) {
	inv, err := scanInvite(r.exec.QueryRow(ctx, getInviteByIdQuery, id))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Invite{}, domain.ErrNotFound
		default:
			return domain.Invite{}, fmt.Errorf("failed to get invite by ID: %w", err)
		}
	}
	return inv, nil
}

// GetInviteByTokenHash looks up an invite by the hash of its raw token,
// returning domain.ErrNotFound if no invite matches.
func (r *PgRepository) GetInviteByTokenHash(ctx context.Context, tokenHash string) (domain.Invite, error) {
	inv, err := scanInvite(r.exec.QueryRow(ctx, getInviteFromTokenHashQuery, tokenHash))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Invite{}, domain.ErrNotFound
		default:
			return domain.Invite{}, fmt.Errorf("failed to get invite: %w", err)
		}

	}
	return inv, nil
}

// GetPendingInviteByEmailAndBusinessId fetches the pending invite
// between a user's email and a business (if it exists)
// and returns the invite.
// It is guaranteed that at most one such pending invite exists
// due to the unique index created on the invites table
// between (email, business_id) where accepted_at is NULL.
func (r *PgRepository) GetPendingInviteByEmailAndBusinessId(
	ctx context.Context,
	email,
	businessId string,
) (domain.Invite, error) {
	inv, err := scanInvite(r.exec.QueryRow(ctx, getPendingInviteByEmailAndBusinessIdQuery, email, businessId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Invite{}, domain.ErrNotFound
		default:
			return domain.Invite{}, fmt.Errorf("failed to get pending invites: %w", err)
		}
	}

	return inv, nil
}

// MarkInviteAccepted stamps accepted_at on the invite matching tokenHash,
// returning domain.ErrNotFound if no invite matches.
func (r *PgRepository) MarkInviteAccepted(ctx context.Context, tokenHash string) error {
	cmdTag, err := r.exec.Exec(ctx, markInviteAcceptedQuery, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to mark invite as accepted: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetPendingInvitesByLocationId returns every not-yet-accepted invite for
// locationId, oldest first, regardless of whether they've since expired
// (callers can compare ExpiresAt themselves to tell the two apart).
func (r *PgRepository) GetPendingInvitesByLocationId(ctx context.Context, locationId string) ([]domain.Invite, error) {
	rows, err := r.exec.Query(ctx, getPendingInvitesByLocationIdQuery, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending invites by location ID: %w", err)
	}
	defer rows.Close()

	invites := []domain.Invite{}
	for rows.Next() {
		var inv domain.Invite
		if err := scanInviteFields(&inv, rows.Scan); err != nil {
			return nil, fmt.Errorf("failed to scan invite: %w", err)
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate invites: %w", err)
	}
	return invites, nil
}

// DeleteInvite deletes the invite matching id, returning domain.ErrNotFound
// if no such invite exists. Callers that need to scope this to a specific
// location (e.g. RevokeInvite) should verify that themselves before calling
// this — it doesn't check on its own.
func (r *PgRepository) DeleteInvite(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteInviteQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.InviteRepository = (*PgRepository)(nil)
