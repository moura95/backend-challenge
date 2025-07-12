package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/user"
)

type UpdateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateUserUseCase struct {
	userRepo user.Repository
}

func NewUpdateUserUseCase(userRepo user.Repository) *UpdateUserUseCase {
	return &UpdateUserUseCase{
		userRepo: userRepo,
	}
}

func (uc *UpdateUserUseCase) Execute(ctx context.Context, userID string, req UpdateUserRequest) (*user.User, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("usecase: update user failed: invalid user ID format")
	}

	foundUser, err := uc.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		return nil, fmt.Errorf("usecase: update user failed: %w", err)
	}

	if strings.TrimSpace(req.Email) != "" && req.Email != foundUser.Email {
		exists, err := uc.userRepo.EmailExists(ctx, req.Email)
		if err != nil {
			return nil, fmt.Errorf("usecase: update user failed: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("usecase: update user failed: email already exists")
		}
	}

	err = foundUser.UpdateUser(req.Name, req.Email)
	if err != nil {
		return nil, fmt.Errorf("usecase: update user failed: %w", err)
	}

	err = uc.userRepo.Update(ctx, foundUser)
	if err != nil {
		return nil, fmt.Errorf("usecase: update user failed: %w", err)
	}

	return foundUser, nil
}
