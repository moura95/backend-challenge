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

type sendWelcomeEmailTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupSendWelcomeEmailTest(t *testing.T) *sendWelcomeEmailTestServer {
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
	err = runSendWelcomeEmailMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &sendWelcomeEmailTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runSendWelcomeEmailMigrations(db *sqlx.DB) error {
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

// Mock Publisher
type MockEmailPublisher struct {
	mock.Mock
}

func (m *MockEmailPublisher) PublishWelcomeEmail(ctx context.Context, data email.WelcomeEmailData) error {
	args := m.Called(ctx, data)
	return args.Error(0)
}

func (m *MockEmailPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestSendWelcomeEmailUseCase_Execute(t *testing.T) {
	server := setupSendWelcomeEmailTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should send welcome email successfully", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "john@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.EmailID)
		assert.Equal(t, "pending", result.Status)
		assert.NotEmpty(t, result.QueuedAt)

		// Verify email was created in database
		var emailCount int
		err = server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE to_email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, emailCount)

		// Verify email details in database
		var dbEmail struct {
			Subject string `db:"subject"`
			Body    string `db:"body"`
			Type    string `db:"type"`
			Status  string `db:"status"`
		}
		err = server.db.Get(&dbEmail, "SELECT subject, body, type, status FROM emails WHERE to_email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Contains(t, dbEmail.Subject, "Welcome")
		assert.Contains(t, dbEmail.Body, "John Doe")
		assert.Equal(t, "welcome", dbEmail.Type)
		assert.Equal(t, "pending", dbEmail.Status)

		// Verify publisher was called
		mockPublisher.AssertExpectations(t)
	})

	t.Run("should fail with empty user email", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request with empty email
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "", // Empty email
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user email is required")

		// Verify publisher was not called
		mockPublisher.AssertNotCalled(t, "PublishWelcomeEmail")
	})

	t.Run("should fail with invalid email format", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request with invalid email
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "invalid-email-format", // Invalid format
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid email format")

		// Verify no email was created
		var emailCount int
		err = server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE to_email = $1", "invalid-email-format")
		require.NoError(t, err)
		assert.Equal(t, 0, emailCount)

		// Verify publisher was not called
		mockPublisher.AssertNotCalled(t, "PublishWelcomeEmail")
	})

	t.Run("should handle publisher failure gracefully", func(t *testing.T) {
		// Setup mock publisher to fail
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(errors.New("queue connection failed"))

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "Queue Fail",
			UserEmail: "queuefail@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to publish welcome email to queue")

		// Verify email was created but marked as failed
		var dbEmail struct {
			Status   string `db:"status"`
			ErrorMsg string `db:"error_msg"`
		}
		err = server.db.Get(&dbEmail, "SELECT status, error_msg FROM emails WHERE to_email = $1", "queuefail@example.com")
		require.NoError(t, err)
		assert.Equal(t, "pending", dbEmail.Status)
		assert.Contains(t, dbEmail.ErrorMsg, "queue connection failed")

		// Verify publisher was called
		mockPublisher.AssertExpectations(t)
	})

	t.Run("should handle nil publisher", func(t *testing.T) {
		// Create use case with nil publisher
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, nil)

		// Test request
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "No Publisher",
			UserEmail: "nopublisher@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "email publisher not configured")

		// Verify email was created but marked as failed
		var dbEmail struct {
			Status   string `db:"status"`
			ErrorMsg string `db:"error_msg"`
		}
		err = server.db.Get(&dbEmail, "SELECT status, error_msg FROM emails WHERE to_email = $1", "nopublisher@example.com")
		require.NoError(t, err)
		assert.Equal(t, "pending", dbEmail.Status)
		assert.Contains(t, dbEmail.ErrorMsg, "email publisher not configured")
	})

	t.Run("should handle special characters in user name", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request with special characters
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "José María Ñoño-García",
			UserEmail: "jose@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify email body contains special characters
		var dbBody string
		err = server.db.Get(&dbBody, "SELECT body FROM emails WHERE to_email = $1", "jose@example.com")
		require.NoError(t, err)
		assert.Contains(t, dbBody, "José María Ñoño-García")

		mockPublisher.AssertExpectations(t)
	})

	t.Run("should handle multiple welcome emails for different users", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil).Times(3)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test requests for multiple users
		users := []SendWelcomeEmailRequest{
			{UserID: uuid.New().String(), UserName: "User 1", UserEmail: "user1@example.com"},
			{UserID: uuid.New().String(), UserName: "User 2", UserEmail: "user2@example.com"},
			{UserID: uuid.New().String(), UserName: "User 3", UserEmail: "user3@example.com"},
		}

		// Execute for all users
		for _, req := range users {
			result, err := useCase.Execute(ctx, req)
			require.NoError(t, err)
			assert.NotNil(t, result)
		}

		// Verify all emails were created
		var emailCount int
		err := server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE type = 'welcome'")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, emailCount, 3)

		mockPublisher.AssertExpectations(t)
	})

	t.Run("should validate email formats correctly", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test invalid email formats
		invalidEmails := []string{
			"plainaddress",
			"@missingdomain.com",
			"missing@.com",
			"missing@domain",
			"spaces in@email.com",
		}

		for _, email := range invalidEmails {
			req := SendWelcomeEmailRequest{
				UserID:    uuid.New().String(),
				UserName:  "Test User",
				UserEmail: email,
			}

			result, err := useCase.Execute(ctx, req)
			assert.Error(t, err, "Email '%s' should be invalid", email)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid email format")
		}

		// Test valid email formats
		validEmails := []string{
			"user@example.com",
			"user.name@example.com",
			"user+tag@example.com",
			"user_name@example.com",
		}

		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil).Times(len(validEmails))

		for _, email := range validEmails {
			req := SendWelcomeEmailRequest{
				UserID:    uuid.New().String(),
				UserName:  "Test User",
				UserEmail: email,
			}

			result, err := useCase.Execute(ctx, req)
			assert.NoError(t, err, "Email '%s' should be valid", email)
			assert.NotNil(t, result)
		}

		mockPublisher.AssertExpectations(t)
	})

	t.Run("should generate correct response format", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Test request
		req := SendWelcomeEmailRequest{
			UserID:    uuid.New().String(),
			UserName:  "Response Test",
			UserEmail: "response@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify response format
		assert.NotEmpty(t, result.EmailID)
		_, err = uuid.Parse(result.EmailID) // Should be valid UUID
		assert.NoError(t, err)

		assert.Equal(t, "pending", result.Status)

		// Verify timestamp format
		assert.NotEmpty(t, result.QueuedAt)
		_, err = time.Parse("2006-01-02T15:04:05Z07:00", result.QueuedAt)
		assert.NoError(t, err, "QueuedAt should be in RFC3339 format")

		mockPublisher.AssertExpectations(t)
	})

	t.Run("should handle concurrent welcome email creation", func(t *testing.T) {
		// Setup mock publisher
		mockPublisher := new(MockEmailPublisher)
		mockPublisher.On("PublishWelcomeEmail", ctx, mock.AnythingOfType("email.WelcomeEmailData")).Return(nil).Times(3)

		// Create use case
		useCase := NewSendWelcomeEmailUseCase(server.repos.Email, mockPublisher)

		// Execute concurrent requests
		done := make(chan bool, 3)
		errors := make(chan error, 3)

		for i := 0; i < 3; i++ {
			go func(id int) {
				req := SendWelcomeEmailRequest{
					UserID:    uuid.New().String(),
					UserName:  fmt.Sprintf("Concurrent User %d", id),
					UserEmail: fmt.Sprintf("concurrent%d@example.com", id),
				}

				_, err := useCase.Execute(ctx, req)
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
		}

		// Wait for all
		for i := 0; i < 3; i++ {
			<-done
		}

		// Check for errors
		select {
		case err := <-errors:
			t.Errorf("Concurrent execution failed: %v", err)
		default:
			// No errors, test passed
		}

		// Verify all emails were created
		var concurrentCount int
		err := server.db.Get(&concurrentCount, "SELECT COUNT(*) FROM emails WHERE to_email LIKE 'concurrent%@example.com'")
		require.NoError(t, err)
		assert.Equal(t, 3, concurrentCount)

		mockPublisher.AssertExpectations(t)
	})
}
