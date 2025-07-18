package user

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, user *User) error

	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	GetByEmail(ctx context.Context, email string) (*User, error)

	Update(ctx context.Context, user *User) error

	Delete(ctx context.Context, id uuid.UUID) error

	List(ctx context.Context, params ListParams) ([]*User, int, error)

	EmailExists(ctx context.Context, email string) (bool, error)
}

type ListParams struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Search   string `json:"search"` // Search by name or email
}
