package handlers

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

	emailUC "github.com/moura95/backend-challenge/internal/application/usecases/email"
	emailDomain "github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
)

type emailConsumerTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupEmailConsumerTest(t *testing.T) *emailConsumerTestServer {
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
	err = runEmailConsumerMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &emailConsumerTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runEmailConsumerMigrations(db *sqlx.DB) error {
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

// Mock Email Service para testar o consumer
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) SendEmail(ctx context.Context, email *emailDomain.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailService) SendEmailDev(ctx context.Context, email *emailDomain.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

func (m *MockEmailService) SendEmailAuto(ctx context.Context, email *emailDomain.Email) error {
	args := m.Called(ctx, email)
	return args.Error(0)
}

// Helper function to create a test email in the database
func createTestEmailForConsumer(t *testing.T, server *emailConsumerTestServer, to, subject string) *emailDomain.Email {
	ctx := context.Background()

	testEmail := &emailDomain.Email{
		ID:          uuid.New(),
		To:          to,
		Subject:     subject,
		Body:        "<h1>Test Email Body</h1>",
		Type:        emailDomain.EmailTypeWelcome,
		Status:      emailDomain.StatusPending,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}

	err := server.repos.Email.Create(ctx, testEmail)
	require.NoError(t, err)

	return testEmail
}

func TestEmailConsumerHandler_HandleEmailMessage(t *testing.T) {
	server := setupEmailConsumerTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should handle email message successfully", func(t *testing.T) {
		// Create test email in database
		testEmail := createTestEmailForConsumer(t, server, "test@example.com", "Test Subject")

		// Setup mock email service to succeed
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message
		message := emailDomain.QueueMessage{
			EmailID: testEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Test User",
				UserEmail: "test@example.com",
			},
		}

		// Execute
		err := handler.HandleEmailMessage(ctx, message)

		// Assert
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify email status in database
		var status string
		err = server.db.Get(&status, "SELECT status FROM emails WHERE uuid = $1", testEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, "sent", status)
	})

	t.Run("should handle email send failure", func(t *testing.T) {
		// Create test email in database
		testEmail := createTestEmailForConsumer(t, server, "fail@example.com", "Fail Subject")

		// Setup mock email service to fail
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(errors.New("SMTP connection failed"))

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message
		message := emailDomain.QueueMessage{
			EmailID: testEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Fail User",
				UserEmail: "fail@example.com",
			},
		}

		// Execute
		err := handler.HandleEmailMessage(ctx, message)

		// Assert - should not error because it can retry
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify email status in database (should still be pending for retry)
		var status string
		var attempts int
		err = server.db.QueryRow("SELECT status, attempts FROM emails WHERE uuid = $1", testEmail.ID).Scan(&status, &attempts)
		require.NoError(t, err)
		assert.Equal(t, "pending", status)
		assert.Equal(t, 1, attempts)
	})

	t.Run("should fail with non-existent email", func(t *testing.T) {
		// Setup mock email service (should not be called)
		mockEmailService := new(MockEmailService)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message with non-existent email ID
		message := emailDomain.QueueMessage{
			EmailID: uuid.New(), // Non-existent
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Non Existent",
				UserEmail: "nonexistent@example.com",
			},
		}

		// Execute
		err := handler.HandleEmailMessage(ctx, message)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to process email message")

		// Email service should not be called
		mockEmailService.AssertNotCalled(t, "SendEmailAuto")
	})

	t.Run("should skip already sent email", func(t *testing.T) {
		// Create test email and mark as sent
		testEmail := createTestEmailForConsumer(t, server, "sent@example.com", "Already Sent")
		testEmail.MarkAsSent()
		err := server.repos.Email.Update(ctx, testEmail)
		require.NoError(t, err)

		// Setup mock email service (should not be called)
		mockEmailService := new(MockEmailService)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message
		message := emailDomain.QueueMessage{
			EmailID: testEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Already Sent",
				UserEmail: "sent@example.com",
			},
		}

		// Execute
		err = handler.HandleEmailMessage(ctx, message)

		// Assert - should not error (just skip)
		require.NoError(t, err)

		// Email service should not be called
		mockEmailService.AssertNotCalled(t, "SendEmailAuto")
	})

	t.Run("should handle email with max attempts reached", func(t *testing.T) {
		// Create test email with max attempts
		testEmail := createTestEmailForConsumer(t, server, "maxattempts@example.com", "Max Attempts")
		testEmail.Attempts = 3
		testEmail.MaxAttempts = 3
		err := server.repos.Email.Update(ctx, testEmail)
		require.NoError(t, err)

		// Setup mock email service (should not be called)
		mockEmailService := new(MockEmailService)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message
		message := emailDomain.QueueMessage{
			EmailID: testEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Max Attempts",
				UserEmail: "maxattempts@example.com",
			},
		}

		// Execute
		err = handler.HandleEmailMessage(ctx, message)

		// Assert - should error because can't retry
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email cannot be retried")

		// Email service should not be called
		mockEmailService.AssertNotCalled(t, "SendEmailAuto")
	})

	t.Run("should handle welcome email with complete data", func(t *testing.T) {
		// Create welcome email using domain logic
		welcomeData := emailDomain.WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "john@example.com",
		}

		welcomeEmail, err := emailDomain.NewWelcomeEmail(welcomeData)
		require.NoError(t, err)

		err = server.repos.Email.Create(ctx, welcomeEmail)
		require.NoError(t, err)

		// Setup mock email service
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Create queue message
		message := emailDomain.QueueMessage{
			EmailID: welcomeEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data:    welcomeData,
		}

		// Execute
		err = handler.HandleEmailMessage(ctx, message)

		// Assert
		require.NoError(t, err)
		mockEmailService.AssertExpectations(t)

		// Verify email content in database
		var subject, body string
		err = server.db.QueryRow("SELECT subject, body FROM emails WHERE uuid = $1", welcomeEmail.ID).Scan(&subject, &body)
		require.NoError(t, err)
		assert.Contains(t, subject, "Welcome")
		assert.Contains(t, body, "John Doe")
	})

	t.Run("should handle multiple messages sequentially", func(t *testing.T) {
		// Create multiple test emails
		email1 := createTestEmailForConsumer(t, server, "multi1@example.com", "Multi 1")
		email2 := createTestEmailForConsumer(t, server, "multi2@example.com", "Multi 2")
		email3 := createTestEmailForConsumer(t, server, "multi3@example.com", "Multi 3")

		// Setup mock email service to succeed for all
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.AnythingOfType("*email.Email")).Return(nil).Times(3)

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Process each email
		emails := []*emailDomain.Email{email1, email2, email3}
		for i, email := range emails {
			message := emailDomain.QueueMessage{
				EmailID: email.ID,
				Type:    emailDomain.EmailTypeWelcome,
				Data: emailDomain.WelcomeEmailData{
					UserID:    uuid.New().String(),
					UserName:  fmt.Sprintf("Multi User %d", i+1),
					UserEmail: email.To,
				},
			}

			err := handler.HandleEmailMessage(ctx, message)
			require.NoError(t, err)
		}

		mockEmailService.AssertExpectations(t)

		// Verify all emails are sent
		var sentCount int
		err := server.db.Get(&sentCount, "SELECT COUNT(*) FROM emails WHERE status = 'sent' AND to_email LIKE 'multi%@example.com'")
		require.NoError(t, err)
		assert.Equal(t, 3, sentCount)
	})

	t.Run("should handle mixed success and failure messages", func(t *testing.T) {
		// Create test emails
		successEmail := createTestEmailForConsumer(t, server, "success@example.com", "Success")
		failEmail := createTestEmailForConsumer(t, server, "fail@example.com", "Fail")

		// Setup mock email service - success for first, fail for second
		mockEmailService := new(MockEmailService)
		mockEmailService.On("SendEmailAuto", ctx, mock.MatchedBy(func(e *emailDomain.Email) bool {
			return e.To == "success@example.com"
		})).Return(nil)
		mockEmailService.On("SendEmailAuto", ctx, mock.MatchedBy(func(e *emailDomain.Email) bool {
			return e.To == "fail@example.com"
		})).Return(errors.New("SMTP error"))

		// Setup use case and handler
		processEmailUC := emailUC.NewProcessEmailQueueUseCase(server.repos.Email, mockEmailService)
		handler := NewEmailConsumerHandler(processEmailUC)

		// Process success email
		successMessage := emailDomain.QueueMessage{
			EmailID: successEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Success User",
				UserEmail: "success@example.com",
			},
		}

		err := handler.HandleEmailMessage(ctx, successMessage)
		require.NoError(t, err)

		// Process fail email
		failMessage := emailDomain.QueueMessage{
			EmailID: failEmail.ID,
			Type:    emailDomain.EmailTypeWelcome,
			Data: emailDomain.WelcomeEmailData{
				UserID:    uuid.New().String(),
				UserName:  "Fail User",
				UserEmail: "fail@example.com",
			},
		}

		err = handler.HandleEmailMessage(ctx, failMessage)
		require.NoError(t, err) // Should not error because it can retry

		mockEmailService.AssertExpectations(t)

		// Verify success email is sent
		var successStatus string
		err = server.db.Get(&successStatus, "SELECT status FROM emails WHERE to_email = 'success@example.com'")
		require.NoError(t, err)
		assert.Equal(t, "sent", successStatus)

		// Verify fail email is pending for retry
		var failStatus string
		var attempts int
		err = server.db.QueryRow("SELECT status, attempts FROM emails WHERE to_email = 'fail@example.com'").Scan(&failStatus, &attempts)
		require.NoError(t, err)
		assert.Equal(t, "pending", failStatus)
		assert.Equal(t, 1, attempts)
	})

}
