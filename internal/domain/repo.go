package domain

import "context"

// Repo aggregates every sub-repository interface defined elsewhere in this
// package into the single interface internal/infra/repo implements against
// Postgres, plus the transaction primitives services use to wrap
// multi-step writes.

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Repo interface {
	// Dedicated (Sub-)Repos
	UserRepository
	RefreshTokenRepository
	BusinessRepository
	LocationRepository
	PositionRepository
	ShiftRepository
	LocationRoleRepository
	BusinessMemberRepository
	EmployeePositionRepository
	InviteRepository
	AuthzContextRepository
	PasswordResetTokenRepository

	// Transaction Functions
	BeginTransaction(ctx context.Context) (Tx, error)
	WithTx(tx Tx) Repo
}
