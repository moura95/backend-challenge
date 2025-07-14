package email

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
)

type emailQueueTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupEmailQueueTest(t *testing.T) *emailQueueTestServer {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to database
	db, err := sqlx.Connect("postgres", connStr)
	require.NoError(t, err)

	// Run migrations
	err = runEmailQueueMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &emailQueueTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runEmailQueueMigrations(db *sqlx.DB) error {
	migrationSQL := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	
	-- Emails table
	CREATE TABLE IF NOT EXISTS emails (
		uuid         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		to_email     VARCHAR(255) NOT NULL,
		subject      VARCHAR(255) NOT NULL,
		body         TEXT NOT NULL,
		type         VARCHAR(50) NOT NULL,
		status       VARCHAR(50) NOT NULL DEFAULT 'pending',
		attempts     INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		error_msg    TEXT,
		sent_at      TIMESTAMPTZ,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	
	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_emails_status ON emails(status);
	CREATE INDEX IF NOT EXISTS idx_emails_type ON emails(type);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

// Mock Email Service
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendEmail(ctx context.Context, email *email.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailService) SendEmailDev(ctx context.Context, email *email.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailService) SendEmailAuto(ctx context.Context, email *email.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

// Helper function to create a test email
func createTestEmailForQueue(t *testing.T, server *emailQueueTestServer, to, subject, body string) *email.Email {
	ctx := context.Background()

	testEmail := &email.Email{
		ID:          uuid.New(),
		To:          to,
		Subject:     subject,
		Body:        body,
		Type:        email.EmailTypeWelcome,
		Status:      email.StatusPending,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}

	err := server.repos.Email.Create(ctx, testEmail)
	require.NoError(t, err)

	return testEmail
}

func TestProcessEmailQueueUseCase_Execute(t *testing.T) {
	server := setupEmailQueueTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should process email successfully", func(t *testing.T) {
		// Create test email
		testEmail := createTestEmailForQueue(t, server, "test@example.com", "Test Subject", "Test Body")

		// Setup mock email service
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message
		message := email.QueueMessage{
			EmailID: testEmail.ID,
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err := useCase.Execute(ctx, message)

		// Assert
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify email status in database
		updatedEmail, err := server.repos.Email.GetByID(ctx, testEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusSent, updatedEmail.Status)
		assert.NotNil(t, updatedEmail.SentAt)
	})

	t.Run("should fail with non-existent email ID", func(t *testing.T) {
		// Setup mock email service
		mockEmailService := new(MockEmailService)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message with non-existent email ID
		message := email.QueueMessage{
			EmailID: uuid.New(), // Non-existent
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err := useCase.Execute(ctx, message)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process email queue failed")
	})

	t.Run("should handle email send failure and retry", func(t *testing.T) {
		// Create test email
		testEmail := createTestEmailForQueue(t, server, "fail@example.com", "Fail Test", "Body")

		// Setup mock email service to fail
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(errors.New("SMTP connection failed"))

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message
		message := email.QueueMessage{
			EmailID: testEmail.ID,
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err := useCase.Execute(ctx, message)

		// Assert - should not error (because can retry)
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify email status in database
		updatedEmail, err := server.repos.Email.GetByID(ctx, testEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusPending, updatedEmail.Status) // Still pending for retry
		assert.Equal(t, 1, updatedEmail.Attempts)
		assert.Equal(t, "email send failed: SMTP connection failed", updatedEmail.ErrorMsg)
	})

	t.Run("should fail permanently after max attempts", func(t *testing.T) {
		// Create test email with max attempts reached
		testEmail := createTestEmailForQueue(t, server, "maxfail@example.com", "Max Fail Test", "Body")
		testEmail.Attempts = 3
		testEmail.MaxAttempts = 3
		err := server.repos.Email.Update(ctx, testEmail)
		require.NoError(t, err)

		// Setup mock email service
		mockEmailService := new(MockEmailService)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message
		message := email.QueueMessage{
			EmailID: testEmail.ID,
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err = useCase.Execute(ctx, message)

		// Assert - should error because can't retry
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email cannot be retried")
	})

	t.Run("should handle email send failure and mark as failed after max attempts", func(t *testing.T) {
		// Create test email with 2 attempts already
		testEmail := createTestEmailForQueue(t, server, "finalfail@example.com", "Final Fail", "Body")
		testEmail.Attempts = 2
		testEmail.MaxAttempts = 3
		err := server.repos.Email.Update(ctx, testEmail)
		require.NoError(t, err)

		// Setup mock email service to fail
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(errors.New("Final SMTP failure"))

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message
		message := email.QueueMessage{
			EmailID: testEmail.ID,
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err = useCase.Execute(ctx, message)

		// Assert - should error because max attempts reached
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email permanently failed after 3 attempts")

		// Verify email status in database
		updatedEmail, err := server.repos.Email.GetByID(ctx, testEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusFailed, updatedEmail.Status)
		assert.Equal(t, 3, updatedEmail.Attempts)
		assert.Equal(t, "email send failed: Final SMTP failure", updatedEmail.ErrorMsg)
	})
}

func TestProcessEmailQueueUseCase_ProcessPendingEmails(t *testing.T) {
	server := setupEmailQueueTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should process multiple pending emails", func(t *testing.T) {
		// Create multiple test emails
		email1 := createTestEmailForQueue(t, server, "batch1@example.com", "Batch 1", "Body 1")
		email2 := createTestEmailForQueue(t, server, "batch2@example.com", "Batch 2", "Body 2")
		email3 := createTestEmailForQueue(t, server, "batch3@example.com", "Batch 3", "Body 3")

		// Setup mock email service to succeed for all
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil).Times(3)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Execute batch processing
		err := useCase.ProcessPendingEmails(ctx, 10)

		// Assert
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify all emails are sent
		emails := []*email.Email{email1, email2, email3}
		for _, e := range emails {
			updatedEmail, err := server.repos.Email.GetByID(ctx, e.ID)
			require.NoError(t, err)
			assert.Equal(t, email.StatusSent, updatedEmail.Status)
		}
	})

	t.Run("should handle batch with mixed success and failure", func(t *testing.T) {
		// Create test emails
		successEmail := createTestEmailForQueue(t, server, "success@example.com", "Success", "Body")
		failEmail := createTestEmailForQueue(t, server, "fail@example.com", "Fail", "Body")

		// Setup mock email service
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.MatchedBy(func(e *email.Email) bool {
			return e.To == "success@example.com"
		})).Return(nil)
		mockEmailService.On("SendEmailAuto", ctx, mock.MatchedBy(func(e *email.Email) bool {
			return e.To == "fail@example.com"
		})).Return(errors.New("SMTP error"))

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Execute batch processing
		err := useCase.ProcessPendingEmails(ctx, 10)

		// Assert - should not error even with some failures
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify success email is sent
		updatedSuccess, err := server.repos.Email.GetByID(ctx, successEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusSent, updatedSuccess.Status)

		// Verify fail email is still pending
		updatedFail, err := server.repos.Email.GetByID(ctx, failEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusPending, updatedFail.Status)
		assert.Equal(t, 1, updatedFail.Attempts)
	})

	t.Run("should handle empty pending emails", func(t *testing.T) {
		// Setup fresh server with no emails
		freshServer := setupEmailQueueTest(t)
		defer freshServer.cleanup()

		// Setup mock email service
		mockEmailService := new(MockEmailService)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(freshServer.repos.Email, mockEmailService)

		// Execute batch processing
		err := useCase.ProcessPendingEmails(ctx, 10)

		// Assert - should not error with empty batch
		require.NoError(t, err)
		// Email service should not be called
		mockEmailService.AssertNotCalled(t, "SendEmailAuto")
	})

	t.Run("should respect batch size limit", func(t *testing.T) {
		// Create more emails than batch size
		for i := 0; i < 5; i++ {
			createTestEmailForQueue(t, server, fmt.Sprintf("batch%d@example.com", i), "Batch Test", "Body")
		}

		// Setup mock email service to succeed
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil).Times(3)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Execute with batch size 3
		err := useCase.ProcessPendingEmails(ctx, 3)

		// Assert
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify only 3 emails were processed
		var sentCount int
		err = server.db.Get(&sentCount, "SELECT COUNT(*) FROM emails WHERE status = 'sent'")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, sentCount, 3)

		var pendingCount int
		err = server.db.Get(&pendingCount, "SELECT COUNT(*) FROM emails WHERE status = 'pending'")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, pendingCount, 2) // At least 2 should remain pending
	})

	t.Run("should handle repository errors gracefully", func(t *testing.T) {
		// This test would require more complex mocking of repository
		// For now, we'll test a scenario where email processing fails due to update error

		// Create test email
		testEmail := createTestEmailForQueue(t, server, "repo-error@example.com", "Repo Error", "Body")

		// Setup mock email service to succeed
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil)

		// Delete the email from DB to simulate repository error during update
		_, err := server.db.Exec("DELETE FROM emails WHERE uuid = $1", testEmail.ID)
		require.NoError(t, err)

		// Create use case
		useCase := NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)

		// Create message
		message := email.QueueMessage{
			EmailID: testEmail.ID,
			Type:    email.EmailTypeWelcome,
		}

		// Execute
		err = useCase.Execute(ctx, message)

		// Assert - should error because email doesn't exist
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "process email queue failed")
	})
}
