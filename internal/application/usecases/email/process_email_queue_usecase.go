package email

import (
	"context"
	"fmt"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/email"
)

type ProcessEmailQueueUseCase struct {
	emailRepo        email.Repository
	emailSender      email.EmailService // Domain service para envio
	maxRetryAttempts int
	retryDelay       time.Duration
}

func NewProcessEmailQueueUseCase(
	emailRepo email.Repository,
	emailSender email.EmailService,
) *ProcessEmailQueueUseCase {
	return &ProcessEmailQueueUseCase{
		emailRepo:        emailRepo,
		emailSender:      emailSender,
		maxRetryAttempts: 3,
		retryDelay:       5 * time.Minute,
	}
}

func (uc *ProcessEmailQueueUseCase) Execute(ctx context.Context, message email.QueueMessage) error {
	emailEntity, err := uc.emailRepo.GetByID(ctx, message.EmailID)
	if err != nil {
		return fmt.Errorf("usecase: process email queue failed: %w", err)
	}

	// 2. Validar se email precisa ser processado
	if err := uc.validateEmailForProcessing(emailEntity); err != nil {
		return err
	}

	// 3. Tentar enviar email
	err = uc.attemptEmailSend(ctx, emailEntity)
	if err != nil {
		// 4. Tratar falha no envio
		return uc.handleSendFailure(ctx, emailEntity, err)
	}

	// 5. Marcar como enviado com sucesso
	return uc.markEmailAsSent(ctx, emailEntity)
}

func (uc *ProcessEmailQueueUseCase) validateEmailForProcessing(emailEntity *email.Email) error {
	if emailEntity.Status == email.StatusSent {
		return nil // Não é erro, apenas ignora
	}

	if !emailEntity.CanRetry() {
		return fmt.Errorf("usecase: process email queue failed: email cannot be retried (attempts: %d/%d)",
			emailEntity.Attempts, emailEntity.MaxAttempts)
	}

	return nil
}

func (uc *ProcessEmailQueueUseCase) attemptEmailSend(ctx context.Context, emailEntity *email.Email) error {
	fmt.Printf("Attempting to send email ID: %s (attempt %d/%d)\n",
		emailEntity.ID.String(), emailEntity.Attempts+1, emailEntity.MaxAttempts)

	emailEntity.Attempts++

	err := uc.emailSender.SendEmail(ctx, emailEntity)
	if err != nil {
		return fmt.Errorf("email send failed: %w", err)
	}

	return nil
}

func (uc *ProcessEmailQueueUseCase) handleSendFailure(ctx context.Context, emailEntity *email.Email, sendErr error) error {
	// Marcar como falha
	emailEntity.MarkAsFailed(sendErr.Error())

	// Persistir o estado de falha
	updateErr := uc.emailRepo.Update(ctx, emailEntity)
	if updateErr != nil {
		return fmt.Errorf("usecase: process email queue failed: send error and update failed. Send error: %w, Update error: %v",
			sendErr, updateErr)
	}

	if emailEntity.CanRetry() {
		fmt.Printf("Email send failed but will retry. Email ID: %s, Error: %v\n",
			emailEntity.ID.String(), sendErr)
		return nil
	}

	return fmt.Errorf("usecase: process email queue failed: email permanently failed after %d attempts: %w",
		emailEntity.MaxAttempts, sendErr)
}

func (uc *ProcessEmailQueueUseCase) markEmailAsSent(ctx context.Context, emailEntity *email.Email) error {
	// Marcar como enviado
	emailEntity.MarkAsSent()

	// Persistir o sucesso
	err := uc.emailRepo.Update(ctx, emailEntity)
	if err != nil {
		return fmt.Errorf("usecase: process email queue failed: update after successful send failed: %w", err)
	}

	fmt.Printf("Email sent successfully. Email ID: %s\n", emailEntity.ID.String())
	return nil
}

func (uc *ProcessEmailQueueUseCase) ProcessPendingEmails(ctx context.Context, batchSize int) error {
	pendingEmails, err := uc.emailRepo.GetPendingEmails(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("usecase: process pending emails failed: %w", err)
	}

	if len(pendingEmails) == 0 {
		return nil // Nenhum email pendente
	}

	successCount := 0
	failureCount := 0

	for _, emailEntity := range pendingEmails {
		message := email.QueueMessage{
			Type:    emailEntity.Type,
			EmailID: emailEntity.ID,
		}

		err := uc.Execute(ctx, message)
		if err != nil {
			failureCount++
			fmt.Printf("Failed to process email ID %s: %v\n", emailEntity.ID.String(), err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Batch processing completed. Success: %d, Failures: %d\n", successCount, failureCount)
	return nil
}
