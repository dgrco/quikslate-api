package domain

// NOTE: This combines all repo-based interfaces into one interface (for argument passing, etc.)
type Repo interface {
	UserRepository
	RefreshTokenRepository
	BusinessRepository
}
