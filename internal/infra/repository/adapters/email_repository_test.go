package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/repository/sqlc"
)

func setupEmailTestDB(t *testing.T) *testDB {
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

	// Run email migrations
	err = runEmailMigrations(db)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &testDB{
		container: postgresContainer,
		db:        db,
		cleanup:   cleanup,
	}
}

func runEmailMigrations(db *sqlx.DB) error {
	// Email table migration for tests
	migrationSQL := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	
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
	
	CREATE INDEX IF NOT EXISTS idx_emails_status ON emails(status);
	CREATE INDEX IF NOT EXISTS idx_emails_type ON emails(type);
	CREATE INDEX IF NOT EXISTS idx_emails_to_email ON emails(to_email);
	CREATE INDEX IF NOT EXISTS idx_emails_created_at ON emails(created_at);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

func createTestEmail() *email.Email {
	return &email.Email{
		ID:          uuid.New(),
		To:          "test@example.com",
		Subject:     "Test Subject",
		Body:        "<h1>Test Email Body</h1>",
		Type:        email.EmailTypeWelcome,
		Status:      email.StatusPending,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}
}

func TestEmailRepository_Create(t *testing.T) {
	testDB := setupEmailTestDB(t)
	defer testDB.cleanup()

	// Setup repository
	queries := sqlc.New(testDB.db)
	repo := NewEmailRepository(queries)

	t.Run("should create email successfully", func(t *testing.T) {
		ctx := context.Background()
		testEmail := createTestEmail()

		// Execute
		err := repo.Create(ctx, testEmail)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, testEmail.ID)
		assert.NotZero(t, testEmail.CreatedAt)
	})

	t.Run("should create welcome email with correct data", func(t *testing.T) {
		ctx := context.Background()

		// Create welcome email using domain method
		welcomeData := email.WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "john@example.com",
		}

		welcomeEmail, err := email.NewWelcomeEmail(welcomeData)
		require.NoError(t, err)

		// Execute
		err = repo.Create(ctx, welcomeEmail)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, email.EmailTypeWelcome, welcomeEmail.Type)
		assert.Equal(t, email.StatusPending, welcomeEmail.Status)
		assert.Equal(t, "john@example.com", welcomeEmail.To)
		assert.Contains(t, welcomeEmail.Subject, "Welcome")
		assert.Contains(t, welcomeEmail.Body, "John Doe")
	})

	t.Run("should handle multiple emails to same address", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple emails to same address
		email1 := createTestEmail()
		email1.Subject = "First Email"

		email2 := createTestEmail()
		email2.Subject = "Second Email"

		// Execute
		err1 := repo.Create(ctx, email1)
		err2 := repo.Create(ctx, email2)

		// Assert
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, email1.ID, email2.ID)
	})
}

func TestEmailRepository_GetByID(t *testing.T) {
	testDB := setupEmailTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewEmailRepository(queries)
	ctx := context.Background()

	// Create test email
	testEmail := createTestEmail()
	err := repo.Create(ctx, testEmail)
	require.NoError(t, err)

	t.Run("should get email by ID", func(t *testing.T) {
		// Execute
		foundEmail, err := repo.GetByID(ctx, testEmail.ID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testEmail.ID, foundEmail.ID)
		assert.Equal(t, testEmail.To, foundEmail.To)
		assert.Equal(t, testEmail.Subject, foundEmail.Subject)
		assert.Equal(t, testEmail.Body, foundEmail.Body)
		assert.Equal(t, testEmail.Type, foundEmail.Type)
		assert.Equal(t, testEmail.Status, foundEmail.Status)
		assert.Equal(t, testEmail.Attempts, foundEmail.Attempts)
		assert.Equal(t, testEmail.MaxAttempts, foundEmail.MaxAttempts)
	})

	t.Run("should return error for non-existent ID", func(t *testing.T) {
		// Execute
		nonExistentID := uuid.New()
		_, err := repo.GetByID(ctx, nonExistentID)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email not found")
	})
}

func TestEmailRepository_Update(t *testing.T) {
	testDB := setupEmailTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewEmailRepository(queries)
	ctx := context.Background()

	// Create test email
	testEmail := createTestEmail()
	err := repo.Create(ctx, testEmail)
	require.NoError(t, err)

	t.Run("should update email status to sent", func(t *testing.T) {
		// Mark as sent
		testEmail.MarkAsSent()

		// Execute
		err := repo.Update(ctx, testEmail)

		// Assert
		require.NoError(t, err)

		// Verify update
		updatedEmail, err := repo.GetByID(ctx, testEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusSent, updatedEmail.Status)
		assert.NotNil(t, updatedEmail.SentAt)
	})

	t.Run("should update email status to failed with error message", func(t *testing.T) {
		// Create new email for this test
		testEmail2 := createTestEmail()
		testEmail2.To = "test2@example.com"
		err := repo.Create(ctx, testEmail2)
		require.NoError(t, err)

		// Mark as failed
		errorMsg := "SMTP connection failed"
		testEmail2.MarkAsFailed(errorMsg)

		// Execute
		err = repo.Update(ctx, testEmail2)

		// Assert
		require.NoError(t, err)

		// Verify update
		updatedEmail, err := repo.GetByID(ctx, testEmail2.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusPending, updatedEmail.Status) // Still pending because attempts < max
		assert.Equal(t, 1, updatedEmail.Attempts)
		assert.Equal(t, errorMsg, updatedEmail.ErrorMsg)
	})

	t.Run("should mark as failed after max attempts", func(t *testing.T) {
		// Create new email for this test
		testEmail3 := createTestEmail()
		testEmail3.To = "test3@example.com"
		testEmail3.MaxAttempts = 2 // Lower max for testing
		err := repo.Create(ctx, testEmail3)
		require.NoError(t, err)

		// Fail twice (should exceed max attempts)
		testEmail3.MarkAsFailed("First failure")
		err = repo.Update(ctx, testEmail3)
		require.NoError(t, err)

		testEmail3.MarkAsFailed("Second failure")
		err = repo.Update(ctx, testEmail3)
		require.NoError(t, err)

		// Verify final status
		updatedEmail, err := repo.GetByID(ctx, testEmail3.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusFailed, updatedEmail.Status)
		assert.Equal(t, 2, updatedEmail.Attempts)
		assert.Equal(t, "Second failure", updatedEmail.ErrorMsg)
	})

}

func TestEmailRepository_Integration_EmailWorkflow(t *testing.T) {
	testDB := setupEmailTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewEmailRepository(queries)
	ctx := context.Background()

	t.Run("complete email workflow", func(t *testing.T) {
		// 1. Create welcome email
		welcomeData := email.WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "Integration Test User",
			UserEmail: "integration@example.com",
		}

		welcomeEmail, err := email.NewWelcomeEmail(welcomeData)
		require.NoError(t, err)

		// 2. Save to database
		err = repo.Create(ctx, welcomeEmail)
		require.NoError(t, err)

		// 3. Retrieve pending emails
		pendingEmails, err := repo.GetPendingEmails(ctx, 10)
		require.NoError(t, err)
		require.Len(t, pendingEmails, 1)
		assert.Equal(t, welcomeEmail.ID, pendingEmails[0].ID)

		// 4. Simulate processing failure
		pendingEmails[0].MarkAsFailed("SMTP timeout")
		err = repo.Update(ctx, pendingEmails[0])
		require.NoError(t, err)

		// 5. Verify still pending (can retry)
		emailAfterFailure, err := repo.GetByID(ctx, welcomeEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusPending, emailAfterFailure.Status)
		assert.Equal(t, 1, emailAfterFailure.Attempts)

		// 6. Simulate successful send
		emailAfterFailure.MarkAsSent()
		err = repo.Update(ctx, emailAfterFailure)
		require.NoError(t, err)

		// 7. Verify final state
		finalEmail, err := repo.GetByID(ctx, welcomeEmail.ID)
		require.NoError(t, err)
		assert.Equal(t, email.StatusSent, finalEmail.Status)
		assert.NotNil(t, finalEmail.SentAt)

		// 8. Verify no longer in pending list
		remainingPending, err := repo.GetPendingEmails(ctx, 10)
		require.NoError(t, err)
		assert.Empty(t, remainingPending)
	})
}
