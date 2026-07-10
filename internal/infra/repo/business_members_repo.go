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
	addUserToBusinessQuery = `
		INSERT INTO business_members (user_id, business_id, is_admin)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, business_id) DO NOTHING
	`
	getBusinessMemberQuery = `
		SELECT user_id, business_id, is_admin, created_at, updated_at
		FROM business_members
		WHERE user_id = $1 AND business_id = $2
	`
	getBusinessMembersByUserIdQuery = `
		SELECT user_id, business_id, is_admin, created_at, updated_at
		FROM business_members
		WHERE user_id = $1
	`
	setAdminForBusinessMemberQuery = `
		UPDATE business_members
		SET is_admin = $3, updated_at = NOW()
		WHERE user_id = $1 AND business_id = $2
	`
	removeUserFromBusinessQuery = `
		DELETE FROM business_members
		WHERE user_id = $1 AND business_id = $2
	`
	// used in removing user to prevent removal of last admin
	getAdminCountQuery = `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM business_members
			WHERE business_id = $1 AND is_admin = TRUE
			FOR UPDATE
		) locked_admins
	`
)

func scanBusinessMemberFields(bm *domain.BusinessMember, scan func(...any) error) error {
	return scan(
		&bm.UserId,
		&bm.BusinessId,
		&bm.IsAdmin,
		&bm.CreatedAt,
		&bm.UpdatedAt,
	)
}

func scanBusinessMember(row pgx.Row) (domain.BusinessMember, error) {
	var bm domain.BusinessMember
	if err := scanBusinessMemberFields(&bm, row.Scan); err != nil {
		return domain.BusinessMember{}, err
	}
	return bm, nil
}

func scanBusinessMembers(rows pgx.Rows) ([]domain.BusinessMember, error) {
	businessMembers := []domain.BusinessMember{}
	for rows.Next() {
		var bm domain.BusinessMember
		if err := scanBusinessMemberFields(&bm, rows.Scan); err != nil {
			return nil, fmt.Errorf("failed to scan business member: %w", err)
		}
		businessMembers = append(businessMembers, bm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate business members: %w", err)
	}
	return businessMembers, nil
}

func (r *PgRepository) AddUserToBusiness(ctx context.Context, userId, businessId string, isAdmin bool) error {
	if _, err := r.exec.Exec(ctx, addUserToBusinessQuery, userId, businessId, isAdmin); err != nil {
		// Check if there is a duplicate
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("failed to add user to business: %w", err)
	}
	return nil
}

func (r *PgRepository) GetBusinessMember(ctx context.Context, userId, businessId string) (domain.BusinessMember, error) {
	bm, err := scanBusinessMember(r.exec.QueryRow(ctx, getBusinessMemberQuery, userId, businessId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.BusinessMember{}, domain.ErrNotFound
		default:
			return domain.BusinessMember{}, fmt.Errorf("failed to get business member: %w", err)
		}
	}
	return bm, nil
}

func (r *PgRepository) GetBusinessMembersByUserId(ctx context.Context, userId string) ([]domain.BusinessMember, error) {
	rows, err := r.exec.Query(ctx, getBusinessMembersByUserIdQuery, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get business members by user ID: %w", err)
	}
	defer rows.Close()

	businessMembers, err := scanBusinessMembers(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get business members by user ID: %w", err)
	}

	return businessMembers, nil
}

func (r *PgRepository) SetAdminForBusinessMember(ctx context.Context, userId, businessId string, admin bool) error {
	cmdTags, err := r.exec.Exec(ctx, setAdminForBusinessMemberQuery, userId, businessId, admin)
	if err != nil {
		return fmt.Errorf("failed to set admin for business member: %w", err)
	}
	if cmdTags.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RemoveUserFromBusiness deletes a user if it does not break the constraint: there must be
// at least one admin per business.
func (r *PgRepository) RemoveUserFromBusiness(ctx context.Context, userId, businessId string) error {
	// ensure last admin is not removable
	tx, err := r.beginPgxTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}
	defer tx.Rollback(ctx)

	bm, err := scanBusinessMember(tx.QueryRow(ctx, getBusinessMemberQuery, userId, businessId))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("failed to remove user from business: %w", err)
	}

	if bm.IsAdmin {
		var nAdmins int
		if err := tx.QueryRow(ctx, getAdminCountQuery, businessId).Scan(&nAdmins); err != nil {
			return fmt.Errorf("failed to remove user from business: %w", err)
		}
		if nAdmins <= 1 {
			return domain.ErrLastAdminRemoval
		}
	}

	// remove the user
	cmdTags, err := tx.Exec(ctx, removeUserFromBusinessQuery, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}
	if cmdTags.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}
	return nil
}

var _ domain.BusinessMemberRepository = (*PgRepository)(nil)
