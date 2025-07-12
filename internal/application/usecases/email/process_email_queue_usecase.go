package email

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/domain/email"
)

type ProcessEmailQueueUseCase struct {
	emailRepo    email.Repository
	emailService email.EmailService
}

func NewProcessEmailQueueUseCase(emailRepo email.Repository, emailService email.EmailService) *ProcessEmailQueueUseCase {
	return &ProcessEmailQueueUseCase{
		emailRepo:    emailRepo,
		emailService: emailService,
	}
}

func (uc *ProcessEmailQueueUseCase) Execute(ctx context.Context, message email.QueueMessage) error {
	emailEntity, err := uc.emailRepo.GetByID(ctx, message.EmailID)
	if err != nil {
		return fmt.Errorf("usecase: process email queue failed: %w", err)
	}

	if emailEntity.Status == email.StatusSent {
		return nil
	}

	if !emailEntity.CanRetry() {
		return fmt.Errorf("usecase: process email queue failed: email cannot be retried")
	}

	err = uc.emailService.SendEmail(ctx, emailEntity)
	if err != nil {
		emailEntity.MarkAsFailed(err.Error())
		updateErr := uc.emailRepo.Update(ctx, emailEntity)
		if updateErr != nil {
			return fmt.Errorf("usecase: process email queue failed: send error and update failed: %w", updateErr)
		}
		return fmt.Errorf("usecase: process email queue failed: %w", err)
	}

	emailEntity.MarkAsSent()
	err = uc.emailRepo.Update(ctx, emailEntity)
	if err != nil {
		return fmt.Errorf("usecase: process email queue failed: update after send failed: %w", err)
	}

	return nil
}
