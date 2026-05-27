package domain

import (
	"context"
	"time"
)

type Business struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BusinessRepository interface {
	CreateBusiness(ctx context.Context, name string) (Business, error)
	GetBusinessById(ctx context.Context, id string) (Business, error)
	ChangeBusinessName(ctx context.Context, id, newName string) error
	DeleteBusiness(ctx context.Context, id string) error
}
