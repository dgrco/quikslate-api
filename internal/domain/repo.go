package domain

import "context"

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

	// Transaction Functions
	BeginTransaction(ctx context.Context) (Tx, error)
	WithTx(tx Tx) Repo
}
