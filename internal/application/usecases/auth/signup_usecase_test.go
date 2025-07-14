package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type testServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupSignUpTest(t *testing.T) *testServer {
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
	err = runSignUpMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &testServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runSignUpMigrations(db *sqlx.DB) error {
	migrationSQL := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	
	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		uuid         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name         VARCHAR(255) NOT NULL,
		email        VARCHAR(100) NOT NULL UNIQUE,
		password     TEXT NOT NULL,
		created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
	);
	
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
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_emails_status ON emails(status);
	CREATE INDEX IF NOT EXISTS idx_emails_type ON emails(type);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

func TestSignUpUseCase_Execute(t *testing.T) {
	server := setupSignUpTest(t)
	defer server.cleanup()

	ctx := context.Background()

	// Setup token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	t.Run("should create user successfully", func(t *testing.T) {
		// Create use case with REAL repositories
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil, // No RabbitMQ for simplicity
		)

		// Test data
		req := SignUpRequest{
			Name:     "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.User)
		assert.Equal(t, "John Doe", result.User.Name)
		assert.Equal(t, "john@example.com", result.User.Email)
		assert.NotEmpty(t, result.User.ID)

		// Verify user in database
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, userCount)

		// Verify email in database
		var emailCount int
		err = server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE to_email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, emailCount)
	})

	t.Run("should fail when email already exists", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// First signup
		req1 := SignUpRequest{
			Name:     "First User",
			Email:    "duplicate@example.com",
			Password: "password123",
		}

		_, err := useCase.Execute(ctx, req1)
		require.NoError(t, err)

		// Second signup with same email
		req2 := SignUpRequest{
			Name:     "Second User",
			Email:    "duplicate@example.com", // Same email
			Password: "password456",
		}

		// Execute
		result, err := useCase.Execute(ctx, req2)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "email already exists")

		// Verify only one user in database
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "duplicate@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, userCount)
	})

	t.Run("should handle invalid email format", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// Test data with invalid email
		req := SignUpRequest{
			Name:     "John Doe",
			Email:    "invalid-email", // Invalid format
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid email format")

		// Verify no user created
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "invalid-email")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)
	})

	t.Run("should handle weak password", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// Test data with weak password
		req := SignUpRequest{
			Name:     "John Doe",
			Email:    "weakpass@example.com",
			Password: "123", // Too short
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "password must be at least 6 characters")

		// Verify no user created
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "weakpass@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)
	})

	t.Run("should handle empty name", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// Test data with empty name
		req := SignUpRequest{
			Name:     "", // Empty name
			Email:    "noname@example.com",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "name must be at least 2 characters")

		// Verify no user created
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "noname@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)
	})

	t.Run("should create multiple users with different emails", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// Create multiple users
		users := []SignUpRequest{
			{Name: "User 1", Email: "user1@example.com", Password: "password123"},
			{Name: "User 2", Email: "user2@example.com", Password: "password123"},
			{Name: "User 3", Email: "user3@example.com", Password: "password123"},
		}

		for _, user := range users {
			result, err := useCase.Execute(ctx, user)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, user.Name, result.User.Name)
			assert.Equal(t, user.Email, result.User.Email)
		}

		// Verify all users in database
		var totalUsers int
		err = server.db.Get(&totalUsers, "SELECT COUNT(*) FROM users")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, totalUsers, 3) // At least 3 from this test

		// Verify all emails in database
		var totalEmails int
		err = server.db.Get(&totalEmails, "SELECT COUNT(*) FROM emails")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, totalEmails, 3) // At least 3 from this test
	})

	t.Run("should handle long name", func(t *testing.T) {
		// Create use case
		useCase := NewSignUpUseCase(
			server.repos.User,
			server.repos.Email,
			tokenMaker,
			nil,
		)

		// Test data with very long name (over 100 chars)
		longName := "This is a very long name that exceeds the maximum allowed length of 100 characters and should be rejected by the validation logic in the domain layer"

		req := SignUpRequest{
			Name:     longName,
			Email:    "longname@example.com",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "name must be less than 100 characters")

		// Verify no user created
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "longname@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)
	})
}
