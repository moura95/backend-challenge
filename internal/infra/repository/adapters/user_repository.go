package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/repository/sqlc"
)

type userRepository struct {
	db *sqlc.Queries
}

func NewUserRepository(db *sqlc.Queries) user.Repository {
	return &userRepository{
		db: db,
	}
}

func (r *userRepository) Create(ctx context.Context, domainUser *user.User) error {
	params := sqlc.CreateUserParams{
		Email:    domainUser.Email,
		Password: domainUser.Password,
		Name:     domainUser.Name,
	}

	sqlcUser, err := r.db.CreateUser(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("repository: create user failed: email already exists")
		}
		return fmt.Errorf("repository: create user failed: %w", err)
	}

	domainUser.ID = sqlcUser.Uuid
	domainUser.CreatedAt = sqlcUser.CreatedAt
	domainUser.UpdatedAt = sqlcUser.UpdatedAt

	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	sqlcUser, err := r.db.GetUserByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository: get user by id failed: user not found")
		}
		return nil, fmt.Errorf("repository: get user by id failed: %w", err)
	}

	return sqlcUserToDomain(sqlcUser), nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	sqlcUser, err := r.db.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository: get user by email failed: user not found")
		}
		return nil, fmt.Errorf("repository: get user by email failed: %w", err)
	}

	return sqlcUserToDomain(sqlcUser), nil
}

func (r *userRepository) Update(ctx context.Context, domainUser *user.User) error {
	params := sqlc.UpdateUserByUUIDParams{
		Uuid: domainUser.ID,
		Name: sql.NullString{
			String: domainUser.Name,
			Valid:  domainUser.Name != "",
		},
		Email: sql.NullString{
			String: domainUser.Email,
			Valid:  domainUser.Email != "",
		},
	}

	err := r.db.UpdateUserByUUID(ctx, params)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("repository: update user failed: user not found")
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("repository: update user failed: email already exists")
		}
		return fmt.Errorf("repository: update user failed: %w", err)
	}

	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.RemoveUserByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("repository: delete user failed: user not found")
		}
		return fmt.Errorf("repository: delete user failed: %w", err)
	}

	return nil
}

func (r *userRepository) List(ctx context.Context, params user.ListParams) ([]*user.User, int, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 10
	}

	offset := (params.Page - 1) * params.PageSize

	listParams := sqlc.ListUsersParams{
		Search: sql.NullString{String: params.Search, Valid: params.Search != ""},
		Limit:  sql.NullInt32{Int32: int32(params.PageSize), Valid: true},
		Offset: sql.NullInt32{Int32: int32(offset), Valid: true},
	}

	sqlcUsers, err := r.db.ListUsers(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("repository: list users failed: %w", err)
	}

	users := make([]*user.User, len(sqlcUsers))
	for i, sqlcUser := range sqlcUsers {
		users[i] = listRowToDomain(sqlcUser)
	}

	return users, len(users), nil
}

func (r *userRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	exists, err := r.db.EmailExists(ctx, email)
	if err != nil {
		return false, fmt.Errorf("repository: email exists check failed: %w", err)
	}

	return exists, nil
}

func sqlcUserToDomain(sqlcUser sqlc.User) *user.User {
	return &user.User{
		ID:        sqlcUser.Uuid,
		Name:      sqlcUser.Name,
		Email:     sqlcUser.Email,
		Password:  sqlcUser.Password,
		CreatedAt: sqlcUser.CreatedAt,
		UpdatedAt: sqlcUser.UpdatedAt,
	}
}

func listRowToDomain(row sqlc.ListUsersRow) *user.User {
	return &user.User{
		ID:        row.Uuid,
		Name:      row.Name,
		Email:     row.Email,
		Password:  "", // Password não vem na listagem por segurança
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
