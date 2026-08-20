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

// ChangePassword is called when the user is authenticated and requires the user to re-enter
// their password for authentication. Once this password is verified, the password is changed.
// This is distinct from ResetPassword (auth.go), which happens if the user is unauthenticated.
// Therefore, this can be called by anyone with at least an Identity session.
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

	// revoke pending reset tokens... otherwise a pending reset token can be used after already changing
	// the password.
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
