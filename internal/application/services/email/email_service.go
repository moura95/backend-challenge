package email

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/application/usecases/email"
	emailDomain "github.com/moura95/backend-challenge/internal/domain/email"
)

type EmailService struct {
	processEmailQueueUseCase *email.ProcessEmailQueueUseCase
}

func NewEmailService(processEmailQueueUC *email.ProcessEmailQueueUseCase) *EmailService {
	return &EmailService{
		processEmailQueueUseCase: processEmailQueueUC,
	}
}

func (s *EmailService) ProcessEmailQueue(ctx context.Context, message emailDomain.QueueMessage) error {
	err := s.processEmailQueueUseCase.Execute(ctx, message)
	if err != nil {
		return fmt.Errorf("service: process email queue failed: %w", err)
	}

	return nil
}
