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
	// 1. Validar entrada
	if err := uc.validateSignInRequest(req); err != nil {
		return nil, fmt.Errorf("usecase: signin failed: %w", err)
	}

	// 2. Buscar usuário por email
	foundUser, err := uc.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("usecase: signin failed: invalid credentials")
	}

	err = foundUser.CheckPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("usecase: signin failed: invalid credentials")
	}

	// 4. Gerar token de autenticação
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

func (uc *SignInUseCase) validateSignInRequest(req SignInRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("email is required")
	}

	if strings.TrimSpace(req.Password) == "" {
		return fmt.Errorf("password is required")
	}

	return nil
}
