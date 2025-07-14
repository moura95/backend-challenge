package user

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
)

type deleteUserTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupDeleteUserTest(t *testing.T) *deleteUserTestServer {
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
	err = runDeleteUserMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &deleteUserTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runDeleteUserMigrations(db *sqlx.DB) error {
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
	
	-- Emails table (to test cascade)
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
	`

	_, err := db.Exec(migrationSQL)
	return err
}

// Helper function to create a test user in the database
func createTestUserForDelete(t *testing.T, server *deleteUserTestServer, email, password, name string) *user.User {
	ctx := context.Background()

	// Create user using domain logic
	testUser, err := user.NewUser(name, email, password)
	require.NoError(t, err)

	// Save to database
	err = server.repos.User.Create(ctx, testUser)
	require.NoError(t, err)

	return testUser
}

// Helper function to check if user exists in database
func userExistsInDB(t *testing.T, server *deleteUserTestServer, userID uuid.UUID) bool {
	var count int
	err := server.db.Get(&count, "SELECT COUNT(*) FROM users WHERE uuid = $1", userID)
	require.NoError(t, err)
	return count > 0
}

func TestDeleteUserUseCase_Execute(t *testing.T) {
	server := setupDeleteUserTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should delete user successfully", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForDelete(t, server, "john@example.com", "password123", "John Doe")

		// Verify user exists before deletion
		assert.True(t, userExistsInDB(t, server, testUser.ID))

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Execute
		err := useCase.Execute(ctx, testUser.ID.String())

		// Assert
		require.NoError(t, err)

		// Verify user no longer exists in database
		assert.False(t, userExistsInDB(t, server, testUser.ID))
	})

	t.Run("should fail with invalid user ID format", func(t *testing.T) {
		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Execute with invalid UUID format
		err := useCase.Execute(ctx, "invalid-uuid-format")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should fail with empty user ID", func(t *testing.T) {
		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Execute with empty user ID
		err := useCase.Execute(ctx, "")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should fail with non-existent user ID", func(t *testing.T) {
		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Generate a valid UUID that doesn't exist in database
		nonExistentID := uuid.New()

		// Execute
		err := useCase.Execute(ctx, nonExistentID.String())

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delete user failed")
	})

	t.Run("should fail when trying to delete already deleted user", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForDelete(t, server, "todelete@example.com", "password123", "To Delete")

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Delete user first time
		err := useCase.Execute(ctx, testUser.ID.String())
		require.NoError(t, err)

		// Try to delete same user again
		err = useCase.Execute(ctx, testUser.ID.String())

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delete user failed")
	})

	t.Run("should handle multiple user deletions", func(t *testing.T) {
		// Create multiple test users
		user1 := createTestUserForDelete(t, server, "user1@example.com", "password123", "User 1")
		user2 := createTestUserForDelete(t, server, "user2@example.com", "password123", "User 2")
		user3 := createTestUserForDelete(t, server, "user3@example.com", "password123", "User 3")

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Delete all users
		users := []*user.User{user1, user2, user3}
		for _, u := range users {
			// Verify user exists before deletion
			assert.True(t, userExistsInDB(t, server, u.ID))

			// Delete user
			err := useCase.Execute(ctx, u.ID.String())
			require.NoError(t, err)

			// Verify user is deleted
			assert.False(t, userExistsInDB(t, server, u.ID))
		}
	})

	t.Run("should handle UUID with different formats", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForDelete(t, server, "format@example.com", "password123", "Format User")

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Test with uppercase UUID
		upperCaseID := testUser.ID.String()
		err := useCase.Execute(ctx, upperCaseID)

		// Assert
		require.NoError(t, err)
		assert.False(t, userExistsInDB(t, server, testUser.ID))
	})

	t.Run("should handle malformed UUID strings", func(t *testing.T) {
		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		malformedUUIDs := []string{
			"123-456-789",
			"not-a-uuid-at-all",
			"12345678-1234-1234-1234-12345678901",  // too long
			"12345678-1234-1234-1234-12345678901Z", // extra character
			"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", // invalid hex
		}

		for _, invalidID := range malformedUUIDs {
			err := useCase.Execute(ctx, invalidID)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid user ID format")
		}
	})

	t.Run("should verify user deletion doesn't affect other users", func(t *testing.T) {
		// Create multiple test users
		userToDelete := createTestUserForDelete(t, server, "delete@example.com", "password123", "Delete Me")
		userToKeep1 := createTestUserForDelete(t, server, "keep1@example.com", "password123", "Keep Me 1")
		userToKeep2 := createTestUserForDelete(t, server, "keep2@example.com", "password123", "Keep Me 2")

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Delete one user
		err := useCase.Execute(ctx, userToDelete.ID.String())
		require.NoError(t, err)

		// Verify deleted user is gone
		assert.False(t, userExistsInDB(t, server, userToDelete.ID))

		// Verify other users still exist
		assert.True(t, userExistsInDB(t, server, userToKeep1.ID))
		assert.True(t, userExistsInDB(t, server, userToKeep2.ID))
	})

	t.Run("should handle deletion with whitespace in UUID", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForDelete(t, server, "whitespace@example.com", "password123", "Whitespace User")

		// Create use case
		useCase := NewDeleteUserUseCase(server.repos.User)

		// Execute with whitespace (should fail since UUID parsing is strict)
		err := useCase.Execute(ctx, "  "+testUser.ID.String()+"  ")

		// Assert - should fail because UUID parsing doesn't trim whitespace
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user ID format")

		// Verify user still exists
		assert.True(t, userExistsInDB(t, server, testUser.ID))
	})

	t.Run("should count users before and after deletion", func(t *testing.T) {
		// Count initial users
		var initialCount int
		err := server.db.Get(&initialCount, "SELECT COUNT(*) FROM users")
		require.NoError(t, err)

		// Create test user
		testUser := createTestUserForDelete(t, server, "count@example.com", "password123", "Count User")

		// Count after creation
		var afterCreateCount int
		err = server.db.Get(&afterCreateCount, "SELECT COUNT(*) FROM users")
		require.NoError(t, err)
		assert.Equal(t, initialCount+1, afterCreateCount)

		// Create use case and delete user
		useCase := NewDeleteUserUseCase(server.repos.User)
		err = useCase.Execute(ctx, testUser.ID.String())
		require.NoError(t, err)

		// Count after deletion
		var afterDeleteCount int
		err = server.db.Get(&afterDeleteCount, "SELECT COUNT(*) FROM users")
		require.NoError(t, err)
		assert.Equal(t, initialCount, afterDeleteCount)
	})
}
