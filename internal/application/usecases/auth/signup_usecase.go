package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type SignUpRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignUpResponse struct {
	User  *user.User `json:"user"`
	Token string     `json:"token"`
}

type SignUpUseCase struct {
	userRepo      user.Repository
	tokenMaker    jwt.Maker
	tokenDuration time.Duration
}

func NewSignUpUseCase(userRepo user.Repository, tokenMaker jwt.Maker) *SignUpUseCase {
	return &SignUpUseCase{
		userRepo:      userRepo,
		tokenMaker:    tokenMaker,
		tokenDuration: 24 * time.Hour, // 24 hours
	}
}

func (uc *SignUpUseCase) Execute(ctx context.Context, req SignUpRequest) (*SignUpResponse, error) {
	exists, err := uc.userRepo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("usecase: signup failed: email already exists")
	}

	newUser, err := user.NewUser(req.Name, req.Email, req.Password)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	err = uc.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	token, _, err := uc.tokenMaker.CreateToken(newUser.ID, uc.tokenDuration)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: token generation error: %w", err)
	}

	response := &SignUpResponse{
		User:  newUser,
		Token: token,
	}

	return response, nil
}
