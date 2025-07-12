package user

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/application/usecases/user"
	domainUser "github.com/moura95/backend-challenge/internal/domain/user"
)

type UserService struct {
	getUserProfileUseCase *user.GetUserProfileUseCase
	updateUserUseCase     *user.UpdateUserUseCase
	deleteUserUseCase     *user.DeleteUserUseCase
	listUsersUseCase      *user.ListUsersUseCase
}

func NewUserService(
	getUserProfileUC *user.GetUserProfileUseCase,
	updateUserUC *user.UpdateUserUseCase,
	deleteUserUC *user.DeleteUserUseCase,
	listUsersUC *user.ListUsersUseCase,
) *UserService {
	return &UserService{
		getUserProfileUseCase: getUserProfileUC,
		updateUserUseCase:     updateUserUC,
		deleteUserUseCase:     deleteUserUC,
		listUsersUseCase:      listUsersUC,
	}
}

func (s *UserService) GetProfile(ctx context.Context, userID string) (*domainUser.User, error) {
	result, err := s.getUserProfileUseCase.Execute(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service: get profile failed: %w", err)
	}

	return result, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID string, req user.UpdateUserRequest) (*domainUser.User, error) {
	result, err := s.updateUserUseCase.Execute(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("service: update profile failed: %w", err)
	}

	return result, nil
}

func (s *UserService) DeleteProfile(ctx context.Context, userID string) error {
	err := s.deleteUserUseCase.Execute(ctx, userID)
	if err != nil {
		return fmt.Errorf("service: delete profile failed: %w", err)
	}

	return nil
}

func (s *UserService) ListUsers(ctx context.Context, req user.ListUsersRequest) (*user.ListUsersResponse, error) {
	result, err := s.listUsersUseCase.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("service: list users failed: %w", err)
	}

	return result, nil
}
