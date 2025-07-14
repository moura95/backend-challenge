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

	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type signInTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupSignInTest(t *testing.T) *signInTestServer {
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
	err = runSignInMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &signInTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runSignInMigrations(db *sqlx.DB) error {
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
	
	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

// Helper function to create a test user in the database
func createTestUser(t *testing.T, server *signInTestServer, email, password, name string) *user.User {
	ctx := context.Background()

	// Create user using domain logic (with proper password hashing)
	testUser, err := user.NewUser(name, email, password)
	require.NoError(t, err)

	// Save to database
	err = server.repos.User.Create(ctx, testUser)
	require.NoError(t, err)

	return testUser
}

func TestSignInUseCase_Execute(t *testing.T) {
	server := setupSignInTest(t)
	defer server.cleanup()

	ctx := context.Background()

	// Setup token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	t.Run("should sign in successfully with valid credentials", func(t *testing.T) {
		// Create test user in database
		testUser := createTestUser(t, server, "john@example.com", "password123", "John Doe")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data
		req := SignInRequest{
			Email:    "john@example.com",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.Token)
		assert.Equal(t, testUser.ID, result.User.ID)
		assert.Equal(t, testUser.Email, result.User.Email)
		assert.Equal(t, testUser.Name, result.User.Name)

		// Verify token is valid
		payload, err := tokenMaker.VerifyToken(result.Token)
		require.NoError(t, err)
		assert.Equal(t, testUser.ID.String(), payload.UserUUID)
	})

	t.Run("should fail with invalid email", func(t *testing.T) {
		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with non-existent email
		req := SignInRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid credentials")
	})

	t.Run("should fail with invalid password", func(t *testing.T) {
		// Create test user in database
		createTestUser(t, server, "jane@example.com", "correctpassword", "Jane Doe")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with wrong password
		req := SignInRequest{
			Email:    "jane@example.com",
			Password: "wrongpassword",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid credentials")
	})

	t.Run("should fail with empty email", func(t *testing.T) {
		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with empty email
		req := SignInRequest{
			Email:    "",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "email is required")
	})

	t.Run("should fail with empty password", func(t *testing.T) {
		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with empty password
		req := SignInRequest{
			Email:    "john@example.com",
			Password: "",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "password is required")
	})

	t.Run("should fail with whitespace-only email", func(t *testing.T) {
		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with whitespace-only email
		req := SignInRequest{
			Email:    "   ",
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "email is required")
	})

	t.Run("should fail with whitespace-only password", func(t *testing.T) {
		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with whitespace-only password
		req := SignInRequest{
			Email:    "john@example.com",
			Password: "   ",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "password is required")
	})

	t.Run("should handle case sensitive passwords", func(t *testing.T) {
		// Create test user in database
		createTestUser(t, server, "case@example.com", "Password123", "Case User")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with different case password
		req := SignInRequest{
			Email:    "case@example.com",
			Password: "password123", // Different case
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid credentials")
	})

	t.Run("should handle email case insensitivity correctly", func(t *testing.T) {
		// Create test user in database
		testUser := createTestUser(t, server, "Mixed@Example.Com", "password123", "Mixed Case User")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data with same email but different case
		req := SignInRequest{
			Email:    "Mixed@Example.Com", // Exact same as stored
			Password: "password123",
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testUser.ID, result.User.ID)
		assert.NotEmpty(t, result.Token)
	})

	t.Run("should generate different tokens for multiple sign-ins", func(t *testing.T) {
		// Create test user in database
		createTestUser(t, server, "multi@example.com", "password123", "Multi User")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data
		req := SignInRequest{
			Email:    "multi@example.com",
			Password: "password123",
		}

		// Execute first sign-in
		result1, err := useCase.Execute(ctx, req)
		require.NoError(t, err)

		// Execute second sign-in
		result2, err := useCase.Execute(ctx, req)
		require.NoError(t, err)

		// Assert
		assert.NotEqual(t, result1.Token, result2.Token)  // Tokens should be different
		assert.Equal(t, result1.User.ID, result2.User.ID) // But same user
	})

	t.Run("should handle special characters in password", func(t *testing.T) {
		// Create test user with special characters in password
		specialPassword := "P@ssw0rd!#$%"
		testUser := createTestUser(t, server, "special@example.com", specialPassword, "Special User")

		// Create use case
		useCase := NewSignInUseCase(server.repos.User, tokenMaker)

		// Test data
		req := SignInRequest{
			Email:    "special@example.com",
			Password: specialPassword,
		}

		// Execute
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testUser.ID, result.User.ID)
		assert.NotEmpty(t, result.Token)
	})
}
