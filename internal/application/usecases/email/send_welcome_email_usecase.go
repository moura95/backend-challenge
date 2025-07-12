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

type SendWelcomeEmailUseCase struct {
	emailRepo email.Repository
	publisher email.Publisher
}

func NewSendWelcomeEmailUseCase(emailRepo email.Repository, publisher email.Publisher) *SendWelcomeEmailUseCase {
	return &SendWelcomeEmailUseCase{
		emailRepo: emailRepo,
		publisher: publisher,
	}
}

func (uc *SendWelcomeEmailUseCase) Execute(ctx context.Context, req SendWelcomeEmailRequest) error {
	data := email.WelcomeEmailData{
		UserID:    req.UserID,
		UserName:  req.UserName,
		UserEmail: req.UserEmail,
	}

	newEmail, err := email.NewWelcomeEmail(data)
	if err != nil {
		return fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	err = uc.emailRepo.Create(ctx, newEmail)
	if err != nil {
		return fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	err = uc.publisher.PublishWelcomeEmail(ctx, data)
	if err != nil {
		return fmt.Errorf("usecase: send welcome email failed: %w", err)
	}

	return nil
}
