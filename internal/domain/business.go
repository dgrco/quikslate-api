package domain

import "time"

type Business struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BusinessRepository interface {
	CreateBusiness(name string) (*Business, error)
	GetBusinessById(id string) (*Business, error)
	ChangeBusinessName(newName string) error
	DeleteBusiness(id string) error
}
