package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type VerifyTokenUseCase struct {
	userRepo   user.Repository
	tokenMaker jwt.Maker
}

func NewVerifyTokenUseCase(userRepo user.Repository, tokenMaker jwt.Maker) *VerifyTokenUseCase {
	return &VerifyTokenUseCase{
		userRepo:   userRepo,
		tokenMaker: tokenMaker,
	}
}

func (uc *VerifyTokenUseCase) Execute(ctx context.Context, token string) (*user.User, error) {
	// 1. Validar entrada
	if token == "" {
		return nil, fmt.Errorf("usecase: verify token failed: token is required")
	}

	// 2. Verificar e decodificar token
	payload, err := uc.tokenMaker.VerifyToken(token)
	if err != nil {
		return nil, fmt.Errorf("usecase: verify token failed: invalid token")
	}

	// 3. Extrair user ID do payload
	userID, err := uuid.Parse(payload.UserUUID)
	if err != nil {
		return nil, fmt.Errorf("usecase: verify token failed: invalid user ID in token")
	}

	foundUser, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("usecase: verify token failed: user not found")
	}
	return foundUser, nil
}
