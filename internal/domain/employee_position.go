package domain

import "context"

type EmployeePosition struct {
	UserId     string `json:"user_id"`
	PositionId string `json:"position_id"`
}

type EmployeePositionRepository interface {
	AddPosition(ctx context.Context, userId, positionId string) error
	RemovePosition(ctx context.Context, userId, positionId string) error
	GetPositionsByUserAndBusiness(ctx context.Context, userId, businessId string) ([]EmployeePosition, error)
	RemoveAllPositionsForUserInBusiness(ctx context.Context, userId, businessId string) error
}
