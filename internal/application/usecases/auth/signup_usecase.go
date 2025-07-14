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
	rabbit        *rabbitmq.Connection
	tokenDuration time.Duration
}

func NewSignUpUseCase(
	userRepo user.Repository,
	emailRepo email.Repository,
	tokenMaker jwt.Maker,
	rabbit *rabbitmq.Connection,
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

	// 4. Criar e salvar email de boas-vindas
	welcomeEmail, err := uc.createWelcomeEmail(newUser)
	if err != nil {
		fmt.Printf("Warning: failed to create welcome email: %v\n", err)
	} else {
		err = uc.emailRepo.Create(ctx, welcomeEmail)
		if err != nil {
			fmt.Printf("Warning: failed to save welcome email: %v\n", err)
		} else {
			// 5. Publicar evento com o ID correto do email
			uc.publishSignUpEvents(ctx, newUser, welcomeEmail)
		}
	}

	// 6. Retornar resposta
	response := &SignUpResponse{
		User: newUser,
	}

	return response, nil
}

func (uc *SignUpUseCase) createWelcomeEmail(user *user.User) (*email.Email, error) {
	welcomeData := email.WelcomeEmailData{
		UserID:    user.ID.String(),
		UserName:  user.Name,
		UserEmail: user.Email,
	}

	return email.NewWelcomeEmail(welcomeData)
}

func (uc *SignUpUseCase) publishSignUpEvents(ctx context.Context, user *user.User, welcomeEmail *email.Email) {
	if uc.rabbit == nil || !uc.rabbit.IsConnected() {
		fmt.Println("Warning: RabbitMQ not available, skipping events")
		return
	}

	welcomeData := email.WelcomeEmailData{
		UserID:    user.ID.String(),
		UserName:  user.Name,
		UserEmail: user.Email,
	}

	message := email.QueueMessage{
		EmailID: welcomeEmail.ID,
		Type:    email.EmailTypeWelcome,
		Data:    welcomeData,
	}

	err := uc.rabbit.PublishWelcomeEmailMessage(message)
	if err != nil {
		fmt.Printf("Warning: failed to publish welcome email: %v\n", err)
	} else {
		fmt.Printf("Published signup events for user %s with email ID %s\n",
			user.Email, welcomeEmail.ID.String())
	}
}
