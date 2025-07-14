package handlers

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/application/usecases/email"
	emailDomain "github.com/moura95/backend-challenge/internal/domain/email"
)

type EmailConsumerHandler struct {
	processEmailUC *email.ProcessEmailQueueUseCase
}

func NewEmailConsumerHandler(processEmailUC *email.ProcessEmailQueueUseCase) *EmailConsumerHandler {
	return &EmailConsumerHandler{
		processEmailUC: processEmailUC,
	}
}

func (h *EmailConsumerHandler) HandleEmailMessage(ctx context.Context, message emailDomain.QueueMessage) error {
	fmt.Printf("Processing email message: %s for user %s\n",
		message.Type, message.Data.UserEmail)

	// Processar a mensagem usando o use case
	err := h.processEmailUC.Execute(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to process email message: %w", err)
	}

	fmt.Printf("Email message processed successfully for user %s\n", message.Data.UserEmail)
	return nil
}
