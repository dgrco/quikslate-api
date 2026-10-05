package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/auth"
	"github.com/dgrco/quikslate/internal/domain"
)

type UserService struct {
	repo domain.Repo
}

func NewUserService(repo domain.Repo) *UserService {
	return &UserService{
		repo,
	}
}

// ChangePassword is the authenticated counterpart to
// AuthService.UsePasswordResetToken, so it re-verifies currentPassword: a
// stolen access token alone must not be enough to take over the account.
func (s *UserService) ChangePassword(ctx context.Context, userId, currentPassword, newPassword string) error {
	if err := domain.ValidateUserPassword(newPassword); err != nil {
		return err
	}

	if err := domain.ValidateUserPasswordChange(currentPassword, newPassword); err != nil {
		return err
	}

	u, err := s.repo.GetUserById(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	if !auth.CheckPassword(currentPassword, u.Password) {
		return domain.ErrInvalidCredentials
	}

	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	tx, err := s.repo.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := s.repo.WithTx(tx)

	// A reset link mailed before this change would otherwise still work after it.
	if err := txRepo.RevokePasswordResetTokensByUserId(ctx, u.Id); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	if err := txRepo.UpdateUser(ctx, u.Id, domain.UserUpdate{Password: &passwordHash}); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	if err := txRepo.RevokeRefreshTokensByUserId(ctx, u.Id); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	return tx.Commit(ctx)
}
