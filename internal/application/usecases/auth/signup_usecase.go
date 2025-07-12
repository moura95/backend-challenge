package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/email"
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
	userRepo       user.Repository
	emailRepo      email.Repository
	tokenMaker     jwt.Maker
	emailPublisher email.Publisher
	tokenDuration  time.Duration
}

func NewSignUpUseCase(
	userRepo user.Repository,
	emailRepo email.Repository,
	tokenMaker jwt.Maker,
	emailPublisher email.Publisher,
) *SignUpUseCase {
	return &SignUpUseCase{
		userRepo:       userRepo,
		emailRepo:      emailRepo,
		tokenMaker:     tokenMaker,
		emailPublisher: emailPublisher,
		tokenDuration:  24 * time.Hour,
	}
}

func (uc *SignUpUseCase) Execute(ctx context.Context, req SignUpRequest) (*SignUpResponse, error) {
	// 1. Validar se email já existe
	exists, err := uc.userRepo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("usecase: signup failed: email already exists")
	}

	// 2. Criar usuário (com validações de domínio)
	newUser, err := user.NewUser(req.Name, req.Email, req.Password)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	// 3. Persistir usuário
	err = uc.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	// 4. Gerar token de autenticação
	token, _, err := uc.tokenMaker.CreateToken(newUser.ID, uc.tokenDuration)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: token generation error: %w", err)
	}

	// 5. Orquestrar envio de email de boas-vindas (assíncrono)
	err = uc.orchestrateWelcomeEmail(ctx, newUser)
	if err != nil {
		// Log o erro mas não falha o signup
		fmt.Printf("Warning: failed to send welcome email: %v\n", err)
	}

	// 6. Retornar resposta
	response := &SignUpResponse{
		User:  newUser,
		Token: token,
	}

	return response, nil
}

func (uc *SignUpUseCase) orchestrateWelcomeEmail(ctx context.Context, user *user.User) error {
	// Criar entidade de email
	data := email.WelcomeEmailData{
		UserID:    user.ID.String(),
		UserName:  user.Name,
		UserEmail: user.Email,
	}

	welcomeEmail, err := email.NewWelcomeEmail(data)
	if err != nil {
		return fmt.Errorf("failed to create welcome email: %w", err)
	}

	// Persistir na base (para auditoria/retry)
	err = uc.emailRepo.Create(ctx, welcomeEmail)
	if err != nil {
		return fmt.Errorf("failed to persist welcome email: %w", err)
	}

	// Publicar na fila para envio assíncrono
	if uc.emailPublisher != nil {
		err = uc.emailPublisher.PublishWelcomeEmail(ctx, data)
		if err != nil {
			return fmt.Errorf("failed to publish welcome email: %w", err)
		}
	}

	return nil
}
