package adapters

import (
	"github.com/jmoiron/sqlx"
	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/repository/sqlc"
)

type Repositories struct {
	User  user.Repository
	Email email.Repository
}

func NewRepositories(db *sqlx.DB) *Repositories {
	queries := sqlc.New(db)

	return &Repositories{
		User:  NewUserRepository(queries),
		Email: NewEmailRepository(queries),
	}
}
