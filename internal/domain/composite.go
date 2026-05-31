package domain

import "context"

// ============================================================================
// This file contains composite function interfaces that write multiple tables.
// ============================================================================

type CompositeRepository interface {
	CreateUserWithBusiness(ctx context.Context, email, password, businessName string) (User, Business, error)
}
