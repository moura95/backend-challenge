package user

import (
	"context"
	"strings"
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
)

type getUserProfileTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupGetUserProfileTest(t *testing.T) *getUserProfileTestServer {
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
	err = runGetUserProfileMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &getUserProfileTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runGetUserProfileMigrations(db *sqlx.DB) error {
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
func createTestUserForProfile(t *testing.T, server *getUserProfileTestServer, email, password, name string) *user.User {
	ctx := context.Background()

	// Create user using domain logic
	testUser, err := user.NewUser(name, email, password)
	require.NoError(t, err)

	// Save to database
	err = server.repos.User.Create(ctx, testUser)
	require.NoError(t, err)

	return testUser
}

func TestGetUserProfileUseCase_Execute(t *testing.T) {
	server := setupGetUserProfileTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should get user profile successfully", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForProfile(t, server, "john@example.com", "password123", "John Doe")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String())

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, testUser.ID, result.ID)
		assert.Equal(t, testUser.Name, result.Name)
		assert.Equal(t, testUser.Email, result.Email)
		assert.Equal(t, testUser.Password, result.Password)
		assert.NotZero(t, result.CreatedAt)
		assert.NotZero(t, result.UpdatedAt)
	})

	t.Run("should fail with invalid user ID format", func(t *testing.T) {
		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute with invalid UUID format
		result, err := useCase.Execute(ctx, "invalid-uuid-format")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should fail with empty user ID", func(t *testing.T) {
		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute with empty user ID
		result, err := useCase.Execute(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should fail with non-existent user ID", func(t *testing.T) {
		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Generate a valid UUID that doesn't exist in database
		nonExistentID := uuid.New()

		// Execute
		result, err := useCase.Execute(ctx, nonExistentID.String())

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "get user profile failed")
	})

	t.Run("should handle multiple users correctly", func(t *testing.T) {
		// Create multiple test users
		user1 := createTestUserForProfile(t, server, "user1@example.com", "password123", "User One")
		user2 := createTestUserForProfile(t, server, "user2@example.com", "password123", "User Two")
		user3 := createTestUserForProfile(t, server, "user3@example.com", "password123", "User Three")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Test each user
		users := []*user.User{user1, user2, user3}
		for _, expectedUser := range users {
			result, err := useCase.Execute(ctx, expectedUser.ID.String())
			require.NoError(t, err)
			assert.Equal(t, expectedUser.ID, result.ID)
			assert.Equal(t, expectedUser.Name, result.Name)
			assert.Equal(t, expectedUser.Email, result.Email)
		}
	})

	t.Run("should handle malformed UUID strings", func(t *testing.T) {
		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		malformedUUIDs := []string{
			"123-456-789",
			"not-a-uuid-at-all",
			"12345678-1234-1234-1234-12345678901",  // too long
			"12345678-1234-1234-1234-12345678901Z", // extra character
			"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", // invalid hex
			"12345678-1234-1234-1234",              // too short
		}

		for _, invalidID := range malformedUUIDs {
			result, err := useCase.Execute(ctx, invalidID)
			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid user ID format")
		}
	})

	t.Run("should handle UUID with different cases", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForProfile(t, server, "case@example.com", "password123", "Case User")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Test with uppercase UUID
		upperCaseID := strings.ToUpper(testUser.ID.String())
		result, err := useCase.Execute(ctx, upperCaseID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testUser.ID, result.ID)
	})

	t.Run("should handle whitespace in UUID", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForProfile(t, server, "whitespace@example.com", "password123", "Whitespace User")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute with whitespace (should fail since UUID parsing is strict)
		result, err := useCase.Execute(ctx, "  "+testUser.ID.String()+"  ")

		// Assert - should fail because UUID parsing doesn't trim whitespace
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should return user with special characters in name", func(t *testing.T) {
		// Create test user with special characters
		specialName := "José María Ñoño-García O'Connor"
		testUser := createTestUserForProfile(t, server, "special@example.com", "password123", specialName)

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, specialName, result.Name)
		assert.Equal(t, testUser.ID, result.ID)
	})

	t.Run("should return user with long email", func(t *testing.T) {
		// Create test user with long email
		longEmail := "very.long.email.address.that.is.still.valid@very-long-domain-name-that-should-work.com"
		testUser := createTestUserForProfile(t, server, longEmail, "password123", "Long Email User")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String())

		// Assert
		require.NoError(t, err)
		assert.Equal(t, longEmail, result.Email)
		assert.Equal(t, testUser.ID, result.ID)
	})

	t.Run("should maintain password security (hashed)", func(t *testing.T) {
		// Create test user
		originalPassword := "mysecretpassword123"
		testUser := createTestUserForProfile(t, server, "security@example.com", originalPassword, "Security User")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String())

		// Assert
		require.NoError(t, err)
		// Password should be hashed, not the original
		assert.NotEqual(t, originalPassword, result.Password)
		assert.NotEmpty(t, result.Password)
		// Should be able to check password correctly
		err = result.CheckPassword(originalPassword)
		assert.NoError(t, err)
	})

	t.Run("should get same user multiple times", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForProfile(t, server, "repeat@example.com", "password123", "Repeat User")

		// Create use case
		useCase := NewGetUserProfileUseCase(server.repos.User)

		// Execute multiple times
		for i := 0; i < 5; i++ {
			result, err := useCase.Execute(ctx, testUser.ID.String())
			require.NoError(t, err)
			assert.Equal(t, testUser.ID, result.ID)
			assert.Equal(t, testUser.Name, result.Name)
			assert.Equal(t, testUser.Email, result.Email)
		}
	})

}
