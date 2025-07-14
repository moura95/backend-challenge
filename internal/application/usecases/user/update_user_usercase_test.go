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

type updateUserTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupUpdateUserTest(t *testing.T) *updateUserTestServer {
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
	err = runUpdateUserMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &updateUserTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runUpdateUserMigrations(db *sqlx.DB) error {
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
func createTestUserForUpdate(t *testing.T, server *updateUserTestServer, email, password, name string) *user.User {
	ctx := context.Background()

	// Create user using domain logic
	testUser, err := user.NewUser(name, email, password)
	require.NoError(t, err)

	// Save to database
	err = server.repos.User.Create(ctx, testUser)
	require.NoError(t, err)

	return testUser
}

func TestUpdateUserUseCase_Execute(t *testing.T) {
	server := setupUpdateUserTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should update user name successfully", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "john@example.com", "password123", "John Doe")
		originalEmail := testUser.Email

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update request
		req := UpdateUserRequest{
			Name:  "John Updated",
			Email: "", // Keep same email
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "John Updated", result.Name)
		assert.Equal(t, originalEmail, result.Email) // Email unchanged
		assert.Equal(t, testUser.ID, result.ID)

		// Verify in database
		var dbName string
		err = server.db.Get(&dbName, "SELECT name FROM users WHERE uuid = $1", testUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "John Updated", dbName)
	})

	t.Run("should update user email successfully", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "jane@example.com", "password123", "Jane Doe")
		originalName := testUser.Name

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update request
		req := UpdateUserRequest{
			Name:  "", // Keep same name
			Email: "jane.updated@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, originalName, result.Name) // Name unchanged
		assert.Equal(t, "jane.updated@example.com", result.Email)
		assert.Equal(t, testUser.ID, result.ID)

		// Verify in database
		var dbEmail string
		err = server.db.Get(&dbEmail, "SELECT email FROM users WHERE uuid = $1", testUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "jane.updated@example.com", dbEmail)
	})

	t.Run("should update both name and email successfully", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "both@example.com", "password123", "Both User")

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update request
		req := UpdateUserRequest{
			Name:  "Both Updated",
			Email: "both.updated@example.com",
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Both Updated", result.Name)
		assert.Equal(t, "both.updated@example.com", result.Email)
		assert.Equal(t, testUser.ID, result.ID)

		// Verify in database
		var dbName, dbEmail string
		err = server.db.QueryRow("SELECT name, email FROM users WHERE uuid = $1", testUser.ID).Scan(&dbName, &dbEmail)
		require.NoError(t, err)
		assert.Equal(t, "Both Updated", dbName)
		assert.Equal(t, "both.updated@example.com", dbEmail)
	})

	t.Run("should fail with invalid user ID format", func(t *testing.T) {
		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update request
		req := UpdateUserRequest{Name: "Test", Email: "test@example.com"}

		// Execute with invalid UUID
		result, err := useCase.Execute(ctx, "invalid-uuid", req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid user ID format")
	})

	t.Run("should fail with non-existent user ID", func(t *testing.T) {
		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update request
		req := UpdateUserRequest{Name: "Test", Email: "test@example.com"}

		// Execute with non-existent UUID
		nonExistentID := uuid.New()
		result, err := useCase.Execute(ctx, nonExistentID.String(), req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "update user failed")
	})

	t.Run("should fail when email already exists", func(t *testing.T) {
		// Create two test users
		user1 := createTestUserForUpdate(t, server, "user1@example.com", "password123", "User 1")
		user2 := createTestUserForUpdate(t, server, "user2@example.com", "password123", "User 2")

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Try to update user1 with user2's email
		req := UpdateUserRequest{
			Name:  "User 1 Updated",
			Email: user2.Email, // This email already exists
		}

		// Execute
		result, err := useCase.Execute(ctx, user1.ID.String(), req)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "email already exists")

		// Verify user1 was not updated
		var dbEmail string
		err = server.db.Get(&dbEmail, "SELECT email FROM users WHERE uuid = $1", user1.ID)
		require.NoError(t, err)
		assert.Equal(t, user1.Email, dbEmail) // Should remain unchanged
	})

	t.Run("should allow updating to same email", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "same@example.com", "password123", "Same User")

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update with same email but different name
		req := UpdateUserRequest{
			Name:  "Same User Updated",
			Email: testUser.Email, // Same email
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Same User Updated", result.Name)
		assert.Equal(t, testUser.Email, result.Email)
	})

	t.Run("should handle empty update request", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "empty@example.com", "password123", "Empty User")
		originalName := testUser.Name
		originalEmail := testUser.Email

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Empty update request
		req := UpdateUserRequest{
			Name:  "",
			Email: "",
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, originalName, result.Name)   // Should remain unchanged
		assert.Equal(t, originalEmail, result.Email) // Should remain unchanged
	})

	t.Run("should validate name length", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "namelength@example.com", "password123", "Name Length")

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Test with name too short
		req1 := UpdateUserRequest{
			Name:  "A", // Too short (< 2 characters)
			Email: "",
		}

		result1, err := useCase.Execute(ctx, testUser.ID.String(), req1)
		assert.Error(t, err)
		assert.Nil(t, result1)
		assert.Contains(t, err.Error(), "name must be at least 2 characters")

		// Test with name too long
		longName := strings.Repeat("A", 101) // Too long (> 100 characters)
		req2 := UpdateUserRequest{
			Name:  longName,
			Email: "",
		}

		result2, err := useCase.Execute(ctx, testUser.ID.String(), req2)
		assert.Error(t, err)
		assert.Nil(t, result2)
		assert.Contains(t, err.Error(), "name must be less than 100 characters")
	})

	t.Run("should handle special characters in name", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "special@example.com", "password123", "Special User")

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update with special characters
		req := UpdateUserRequest{
			Name:  "José María Ñoño-García O'Connor",
			Email: "",
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "José María Ñoño-García O'Connor", result.Name)

		// Verify in database
		var dbName string
		err = server.db.Get(&dbName, "SELECT name FROM users WHERE uuid = $1", testUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "José María Ñoño-García O'Connor", dbName)
	})

	t.Run("should update updated_at timestamp", func(t *testing.T) {
		// Create test user
		testUser := createTestUserForUpdate(t, server, "timestamp@example.com", "password123", "Timestamp User")
		originalUpdatedAt := testUser.UpdatedAt

		// Wait a bit to ensure timestamp difference
		time.Sleep(100 * time.Millisecond)

		// Create use case
		useCase := NewUpdateUserUseCase(server.repos.User)

		// Update user
		req := UpdateUserRequest{
			Name:  "Timestamp Updated",
			Email: "",
		}

		// Execute
		result, err := useCase.Execute(ctx, testUser.ID.String(), req)

		// Assert
		require.NoError(t, err)
		assert.True(t, result.UpdatedAt.After(originalUpdatedAt), "UpdatedAt should be updated")

		// Verify in database
		var dbUpdatedAt time.Time
		err = server.db.Get(&dbUpdatedAt, "SELECT updated_at FROM users WHERE uuid = $1", testUser.ID)
		require.NoError(t, err)
		assert.True(t, dbUpdatedAt.After(originalUpdatedAt), "Database UpdatedAt should be updated")
	})

}
