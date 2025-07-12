package user

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/domain/user"
)

type ListUsersRequest struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Search   string `json:"search"`
}

type ListUsersResponse struct {
	Users []*user.User `json:"users"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
}

type ListUsersUseCase struct {
	userRepo user.Repository
}

func NewListUsersUseCase(userRepo user.Repository) *ListUsersUseCase {
	return &ListUsersUseCase{
		userRepo: userRepo,
	}
}

func (uc *ListUsersUseCase) Execute(ctx context.Context, req ListUsersRequest) (*ListUsersResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	params := user.ListParams{
		Page:     req.Page,
		PageSize: req.PageSize,
		Search:   req.Search,
	}

	users, total, err := uc.userRepo.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("usecase: list users failed: %w", err)
	}

	response := &ListUsersResponse{
		Users: users,
		Total: total,
		Page:  req.Page,
	}

	return response, nil
}
