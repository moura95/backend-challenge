package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/user"
)

type DeleteUserUseCase struct {
	userRepo user.Repository
}

func NewDeleteUserUseCase(userRepo user.Repository) *DeleteUserUseCase {
	return &DeleteUserUseCase{
		userRepo: userRepo,
	}
}

func (uc *DeleteUserUseCase) Execute(ctx context.Context, userID string) error {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("usecase: delete user failed: invalid user ID format")
	}

	_, err = uc.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		return fmt.Errorf("usecase: delete user failed: %w", err)
	}

	err = uc.userRepo.Delete(ctx, parsedID)
	if err != nil {
		return fmt.Errorf("usecase: delete user failed: %w", err)
	}

	return nil
}
