package auth

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

	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
)

type verifyTokenTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupVerifyTokenTest(t *testing.T) *verifyTokenTestServer {
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
	err = runVerifyTokenMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &verifyTokenTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runVerifyTokenMigrations(db *sqlx.DB) error {
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

// Helper function to create a test user and return a valid token
func createUserAndToken(t *testing.T, server *verifyTokenTestServer, tokenMaker jwt.Maker, email, password, name string) (*user.User, string) {
	ctx := context.Background()

	// Create user using domain logic
	testUser, err := user.NewUser(name, email, password)
	require.NoError(t, err)

	// Save to database
	err = server.repos.User.Create(ctx, testUser)
	require.NoError(t, err)

	// Generate token for this user
	token, _, err := tokenMaker.CreateToken(testUser.ID, 24*time.Hour)
	require.NoError(t, err)

	return testUser, token
}

func TestVerifyTokenUseCase_Execute(t *testing.T) {
	server := setupVerifyTokenTest(t)
	defer server.cleanup()

	ctx := context.Background()

	// Setup token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	t.Run("should verify token successfully with valid token", func(t *testing.T) {
		// Create test user and get token
		testUser, validToken := createUserAndToken(t, server, tokenMaker, "john@example.com", "password123", "John Doe")

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute
		result, err := useCase.Execute(ctx, validToken)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testUser.ID, result.ID)
		assert.Equal(t, testUser.Email, result.Email)
		assert.Equal(t, testUser.Name, result.Name)
	})

	t.Run("should fail with empty token", func(t *testing.T) {
		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with empty token
		result, err := useCase.Execute(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "token is required")
	})

	t.Run("should fail with invalid token format", func(t *testing.T) {
		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with invalid token
		result, err := useCase.Execute(ctx, "invalid.token.format")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("should fail with expired token", func(t *testing.T) {
		// Create test user
		testUser, err := user.NewUser("Jane Doe", "jane@example.com", "password123")
		require.NoError(t, err)

		err = server.repos.User.Create(ctx, testUser)
		require.NoError(t, err)

		// Generate expired token (negative duration)
		expiredToken, _, err := tokenMaker.CreateToken(testUser.ID, -1*time.Hour)
		require.NoError(t, err)

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with expired token
		result, err := useCase.Execute(ctx, expiredToken)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("should fail with token for non-existent user", func(t *testing.T) {
		// Generate token for non-existent user
		fakeUserID := uuid.New()
		fakeToken, _, err := tokenMaker.CreateToken(fakeUserID, 24*time.Hour)
		require.NoError(t, err)

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with token for non-existent user
		result, err := useCase.Execute(ctx, fakeToken)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("should fail with malformed token", func(t *testing.T) {
		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with malformed token
		result, err := useCase.Execute(ctx, "clearly.not.a.valid.jwt.token.format")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("should handle multiple users with valid tokens", func(t *testing.T) {
		// Create multiple users and tokens
		user1, token1 := createUserAndToken(t, server, tokenMaker, "user1@example.com", "password123", "User 1")
		user2, token2 := createUserAndToken(t, server, tokenMaker, "user2@example.com", "password123", "User 2")
		user3, token3 := createUserAndToken(t, server, tokenMaker, "user3@example.com", "password123", "User 3")

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Test each token
		testCases := []struct {
			user  *user.User
			token string
		}{
			{user1, token1},
			{user2, token2},
			{user3, token3},
		}

		for _, tc := range testCases {
			result, err := useCase.Execute(ctx, tc.token)
			require.NoError(t, err)
			assert.Equal(t, tc.user.ID, result.ID)
			assert.Equal(t, tc.user.Email, result.Email)
			assert.Equal(t, tc.user.Name, result.Name)
		}
	})

	t.Run("should handle token with whitespace", func(t *testing.T) {
		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with whitespace token
		result, err := useCase.Execute(ctx, "   ")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("should verify token multiple times for same user", func(t *testing.T) {
		// Create test user and get token
		testUser, validToken := createUserAndToken(t, server, tokenMaker, "repeat@example.com", "password123", "Repeat User")

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute multiple times
		for i := 0; i < 3; i++ {
			result, err := useCase.Execute(ctx, validToken)
			require.NoError(t, err)
			assert.Equal(t, testUser.ID, result.ID)
		}
	})

	t.Run("should handle user deleted after token creation", func(t *testing.T) {
		// Create test user and get token
		testUser, validToken := createUserAndToken(t, server, tokenMaker, "todelete@example.com", "password123", "To Delete User")

		// Delete user from database
		err := server.repos.User.Delete(ctx, testUser.ID)
		require.NoError(t, err)

		// Create use case
		useCase := NewVerifyTokenUseCase(server.repos.User, tokenMaker)

		// Execute with token for deleted user
		result, err := useCase.Execute(ctx, validToken)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user not found")
	})
}
