package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/user"
)

type GetUserProfileUseCase struct {
	userRepo user.Repository
}

func NewGetUserProfileUseCase(userRepo user.Repository) *GetUserProfileUseCase {
	return &GetUserProfileUseCase{
		userRepo: userRepo,
	}
}

func (uc *GetUserProfileUseCase) Execute(ctx context.Context, userID string) (*user.User, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("usecase: get user profile failed: invalid user ID format")
	}

	foundUser, err := uc.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		return nil, fmt.Errorf("usecase: get user profile failed: %w", err)
	}

	return foundUser, nil
}
