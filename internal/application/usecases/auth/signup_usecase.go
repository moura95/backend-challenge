package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
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
	emailRepo     email.Repository
	tokenMaker    jwt.Maker
	rabbit        *rabbitmq.Connection // 👈 SÓ o RabbitMQ!
	tokenDuration time.Duration
}

func NewSignUpUseCase(
	userRepo user.Repository,
	emailRepo email.Repository,
	tokenMaker jwt.Maker,
	rabbit *rabbitmq.Connection, // 👈 Uma dependência só!
) *SignUpUseCase {
	return &SignUpUseCase{
		userRepo:      userRepo,
		emailRepo:     emailRepo,
		tokenMaker:    tokenMaker,
		rabbit:        rabbit,
		tokenDuration: 24 * time.Hour,
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

	// 2. Criar usuário
	newUser, err := user.NewUser(req.Name, req.Email, req.Password)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	// 3. Persistir usuário
	err = uc.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: %w", err)
	}

	// 4. Gerar token
	token, _, err := uc.tokenMaker.CreateToken(newUser.ID, uc.tokenDuration)
	if err != nil {
		return nil, fmt.Errorf("usecase: signup failed: token generation error: %w", err)
	}

	uc.publishSignUpEvents(ctx, newUser)

	// 6. Retornar resposta
	response := &SignUpResponse{
		User:  newUser,
		Token: token,
	}

	return response, nil
}

func (uc *SignUpUseCase) publishSignUpEvents(ctx context.Context, user *user.User) {
	if uc.rabbit == nil || !uc.rabbit.IsConnected() {
		fmt.Println("Warning: RabbitMQ not available, skipping events")
		return
	}

	welcomeData := email.WelcomeEmailData{
		UserID:    user.ID.String(),
		UserName:  user.Name,
		UserEmail: user.Email,
	}

	err := uc.rabbit.PublishWelcomeEmail(ctx, welcomeData)
	if err != nil {
		fmt.Printf("Warning: failed to publish welcome email: %v\n", err)
	}

	fmt.Printf("Published signup events for user %s\n", user.Email)
}
