package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignInResponse struct {
	User  *user.User `json:"user"`
	Token string     `json:"token"`
}

type SignInUseCase struct {
	userRepo      user.Repository
	tokenMaker    jwt.Maker
	tokenDuration time.Duration
}

func NewSignInUseCase(userRepo user.Repository, tokenMaker jwt.Maker) *SignInUseCase {
	return &SignInUseCase{
		userRepo:      userRepo,
		tokenMaker:    tokenMaker,
		tokenDuration: 24 * time.Hour, // 24 hours
	}
}

func (uc *SignInUseCase) Execute(ctx context.Context, req SignInRequest) (*SignInResponse, error) {
	if strings.TrimSpace(req.Email) == "" {
		return nil, fmt.Errorf("usecase: signin failed: email is required")
	}

	if strings.TrimSpace(req.Password) == "" {
		return nil, fmt.Errorf("usecase: signin failed: password is required")
	}

	foundUser, err := uc.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("usecase: signin failed: invalid credentials")
	}

	err = foundUser.CheckPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("usecase: signin failed: invalid credentials")
	}

	token, _, err := uc.tokenMaker.CreateToken(foundUser.ID, uc.tokenDuration)
	if err != nil {
		return nil, fmt.Errorf("usecase: signin failed: token generation error: %w", err)
	}

	response := &SignInResponse{
		User:  foundUser,
		Token: token,
	}

	return response, nil
}
