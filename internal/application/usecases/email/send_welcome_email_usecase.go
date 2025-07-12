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

	// 3. Persistir na base (para auditoria e controle)
	err = uc.emailRepo.Create(ctx, emailEntity)
	if err != nil {
		return nil, fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	// 4. Orquestrar envio (síncrono ou assíncrono)
	err = uc.orchestrateEmailDelivery(ctx, emailEntity, req)
	if err != nil {
		// Se falhar, marcar email como falha mas não falhar o use case completo
		uc.handleDeliveryFailure(ctx, emailEntity, err)
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
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	if req.UserName == "" {
		return fmt.Errorf("user name is required")
	}

	if req.UserEmail == "" {
		return fmt.Errorf("user email is required")
	}

	// Validação adicional de email usando domain validator
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

func (uc *SendWelcomeEmailUseCase) orchestrateEmailDelivery(ctx context.Context, emailEntity *email.Email, req SendWelcomeEmailRequest) error {
	return uc.sendEmailAsynchronously(ctx, emailEntity, req)
}

func (uc *SendWelcomeEmailUseCase) sendEmailAsynchronously(ctx context.Context, emailEntity *email.Email, req SendWelcomeEmailRequest) error {
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

	fmt.Printf("Welcome email queued for delivery: %s\n", emailEntity.ID.String())
	return nil
}

func (uc *SendWelcomeEmailUseCase) handleDeliveryFailure(ctx context.Context, emailEntity *email.Email, deliveryErr error) {
	// Marcar como falha
	emailEntity.MarkAsFailed(deliveryErr.Error())

	// Tentar atualizar na base
	updateErr := uc.emailRepo.Update(ctx, emailEntity)
	if updateErr != nil {
		fmt.Printf("Failed to update email status after delivery failure. Email ID: %s, Delivery error: %v, Update error: %v\n",
			emailEntity.ID.String(), deliveryErr, updateErr)
	}
}

func (uc *SendWelcomeEmailUseCase) SendWelcomeEmailBatch(ctx context.Context, requests []SendWelcomeEmailRequest) ([]*SendWelcomeEmailResponse, error) {
	responses := make([]*SendWelcomeEmailResponse, 0, len(requests))

	for _, req := range requests {
		response, err := uc.Execute(ctx, req)
		if err != nil {
			// Log erro mas continua processando os outros
			fmt.Printf("Failed to send welcome email for user %s: %v\n", req.UserID, err)
			continue
		}
		responses = append(responses, response)
	}

	return responses, nil
}
