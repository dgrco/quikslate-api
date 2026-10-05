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
		INSERT INTO business_members (user_id, business_id, is_primary_admin, is_admin)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, business_id) DO NOTHING
	`
	getBusinessMemberQuery = `
		SELECT user_id, business_id, is_primary_admin, is_admin, created_at, updated_at
		FROM business_members
		WHERE user_id = $1 AND business_id = $2
	`
	getBusinessMemberDetailsByBusinessId = `
		SELECT bm.user_id, bm.business_id, u.name, u.email, bm.is_primary_admin, bm.is_admin, bm.created_at, bm.updated_at
		FROM business_members bm
		JOIN users u ON bm.user_id = u.id
		WHERE bm.business_id = $1
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
		&bm.IsPrimaryAdmin,
		&bm.IsAdmin,
		&bm.CreatedAt,
		&bm.UpdatedAt,
	)
}

func scanBusinessMemberDetailFields(bmd *domain.BusinessMemberDetail, scan func(...any) error) error {
	return scan(
		&bmd.UserId,
		&bmd.BusinessId,
		&bmd.Name,
		&bmd.Email,
		&bmd.IsPrimaryAdmin,
		&bmd.IsAdmin,
		&bmd.CreatedAt,
		&bmd.UpdatedAt,
	)
}

func scanBusinessMember(row pgx.Row) (domain.BusinessMember, error) {
	var bm domain.BusinessMember
	if err := scanBusinessMemberFields(&bm, row.Scan); err != nil {
		return domain.BusinessMember{}, err
	}
	return bm, nil
}

func scanBusinessMemberDetails(rows pgx.Rows) ([]domain.BusinessMemberDetail, error) {
	bmds := []domain.BusinessMemberDetail{}
	for rows.Next() {
		var bmd domain.BusinessMemberDetail
		err := scanBusinessMemberDetailFields(&bmd, rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan business member details: %w", err)
		}
		bmds = append(bmds, bmd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate business member details: %w", err)
	}
	return bmds, nil
}

// AddUserToBusiness inserts a business_members row for userId at businessId.
// Returns domain.ErrAlreadyExists (instead of a raw unique-constraint error)
// if userId is already a member of businessId.
func (r *PgRepository) AddUserToBusiness(
	ctx context.Context,
	userId,
	businessId string,
	isPrimaryAdmin,
	isAdmin bool,
) error {
	if _, err := r.exec.Exec(ctx, addUserToBusinessQuery, userId, businessId, isPrimaryAdmin, isAdmin); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("failed to add user to business: %w", err)
	}
	return nil
}

// GetBusinessMember fetches userId's membership row for businessId,
// returning domain.ErrNotFound if they aren't a member.
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

// GetBusinessMemberDetailsByBusinessId returns every member of businessId,
// each joined with their user record (name, email).
func (r *PgRepository) GetBusinessMemberDetailsByBusinessId(ctx context.Context, businessId string) ([]domain.BusinessMemberDetail, error) {
	rows, err := r.exec.Query(ctx, getBusinessMemberDetailsByBusinessId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get business member details: %w", err)
	}
	defer rows.Close()

	bmds, err := scanBusinessMemberDetails(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get business member details: %w", err)
	}

	return bmds, nil
}

// SetAdminForBusinessMember flips a member's is_admin flag, returning
// domain.ErrNotFound if userId isn't a member of businessId.
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

// GetAdminCount counts businessId's admins. The underlying query locks the
// matching rows with FOR UPDATE, so callers running this inside a
// transaction (e.g. before demoting the last remaining admin) can safely act
// on the count without a concurrent demotion racing them.
func (r *PgRepository) GetAdminCount(ctx context.Context, businessId string) (int, error) {
	var nAdmins int
	if err := r.exec.QueryRow(ctx, getAdminCountQuery, businessId).Scan(&nAdmins); err != nil {
		return 0, fmt.Errorf("failed to get admin count: %w", err)
	}
	return nAdmins, nil
}

// RemoveUserFromBusiness deletes a member's business_members row, returning
// domain.ErrNotFound if userId isn't a member of businessId.
func (r *PgRepository) RemoveUserFromBusiness(ctx context.Context, userId, businessId string) error {
	cmdTags, err := r.exec.Exec(ctx, removeUserFromBusinessQuery, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove user from business: %w", err)
	}
	if cmdTags.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.BusinessMemberRepository = (*PgRepository)(nil)
