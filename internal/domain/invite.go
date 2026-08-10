package domain

import (
	"context"
	"time"
)

// Invite represents a pending invitation for someone to join a business at a
// specific location and role. Accepting one creates the matching
// BusinessMember and LocationRole rows in a single transaction, see
// internal/service/invite.go.

type Invite struct {
	Id         string     `json:"id"`
	TokenHash  string     `json:"token_hash"`
	Email      string     `json:"email"`
	BusinessId string     `json:"business_id"`
	LocationId string     `json:"location_id"`
	Role       LRole      `json:"role"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ValidateInviteExpiration returns an error if expiresAt is already in the
// past. Not yet wired into the create-invite path; kept for when expiry
// becomes user-configurable.
func ValidateInviteExpiration(expiresAt time.Time) error {
	if time.Now().After(expiresAt) {
		return NewValidationError("new invite's expiration must not be in the past")
	}
	return nil
}

type InviteRepository interface {
	CreateInvite(ctx context.Context, tokenHash, email, businessId, locationId string, role LRole, expiresAt time.Time) (Invite, error)
	GetInviteById(ctx context.Context, id string) (Invite, error)
	GetInviteByTokenHash(ctx context.Context, tokenHash string) (Invite, error)
	GetPendingInviteByEmailAndBusinessId(ctx context.Context, email, businessId string) (Invite, error)
	MarkInviteAccepted(ctx context.Context, tokenHash string) error
	GetPendingInvitesByLocationId(ctx context.Context, locationId string) ([]Invite, error)
	DeleteInvite(ctx context.Context, id string) error
}
