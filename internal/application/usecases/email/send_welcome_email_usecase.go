package email

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/domain/email"
)

type SendWelcomeEmailRequest struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

type SendWelcomeEmailResponse struct {
	EmailID  string `json:"email_id"`
	Status   string `json:"status"`
	QueuedAt string `json:"queued_at"`
}

type SendWelcomeEmailUseCase struct {
	emailRepo email.Repository
	publisher email.Publisher
}

func NewSendWelcomeEmailUseCase(
	emailRepo email.Repository,
	publisher email.Publisher,
) *SendWelcomeEmailUseCase {
	return &SendWelcomeEmailUseCase{
		emailRepo: emailRepo,
		publisher: publisher,
	}
}

func (uc *SendWelcomeEmailUseCase) Execute(ctx context.Context, req SendWelcomeEmailRequest) (*SendWelcomeEmailResponse, error) {
	// 1. Validar request
	if err := uc.validateRequest(req); err != nil {
		return nil, fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	// 2. Criar entidade de email
	emailEntity, err := uc.createWelcomeEmail(req)
	if err != nil {
		return nil, fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	// 3. Salvar no banco
	err = uc.emailRepo.Create(ctx, emailEntity)
	if err != nil {
		return nil, fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	// 4. Enviar para fila
	err = uc.sendToQueue(ctx, req)
	if err != nil {
		// Se falhar, marcar como falha
		emailEntity.MarkAsFailed(err.Error())
		uc.emailRepo.Update(ctx, emailEntity)
		return nil, fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	// 5. Retornar resposta
	response := &SendWelcomeEmailResponse{
		EmailID:  emailEntity.ID.String(),
		Status:   string(emailEntity.Status),
		QueuedAt: emailEntity.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	return response, nil
}

func (uc *SendWelcomeEmailUseCase) validateRequest(req SendWelcomeEmailRequest) error {
	if req.UserName == "" {
		return fmt.Errorf("user name is required")
	}

	if req.UserEmail == "" {
		return fmt.Errorf("user email is required")
	}

	// Validação de email
	validator := email.NewEmailValidator()
	if err := validator.ValidateEmail(req.UserEmail); err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}

	return nil
}

func (uc *SendWelcomeEmailUseCase) createWelcomeEmail(req SendWelcomeEmailRequest) (*email.Email, error) {
	data := email.WelcomeEmailData{
		UserID:    req.UserID,
		UserName:  req.UserName,
		UserEmail: req.UserEmail,
	}

	welcomeEmail, err := email.NewWelcomeEmail(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create welcome email entity: %w", err)
	}

	return welcomeEmail, nil
}

func (uc *SendWelcomeEmailUseCase) sendToQueue(ctx context.Context, req SendWelcomeEmailRequest) error {
	if uc.publisher == nil {
		return fmt.Errorf("email publisher not configured")
	}

	data := email.WelcomeEmailData{
		UserID:    req.UserID,
		UserName:  req.UserName,
		UserEmail: req.UserEmail,
	}

	err := uc.publisher.PublishWelcomeEmail(ctx, data)
	if err != nil {
		return fmt.Errorf("failed to publish welcome email to queue: %w", err)
	}

	fmt.Printf("Welcome email queued for delivery: %s\n", req.UserEmail)
	return nil
}
