package adapters

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
	"github.com/moura95/backend-challenge/internal/infra/repository/sqlc"
)

type testDB struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	cleanup   func()
}

func setupTestDB(t *testing.T) *testDB {
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
	err = runMigrations(db)
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

func runMigrations(db *sqlx.DB) error {
	// Simple migration for tests
	migrationSQL := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	
	CREATE TABLE IF NOT EXISTS users (
		uuid         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name         VARCHAR(255) NOT NULL,
		email        VARCHAR(100) NOT NULL UNIQUE,
		password     TEXT NOT NULL,
		created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at   TIMESTAMP NOT NULL DEFAULT NOW()
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

func TestUserRepository_Create(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	// Setup repository
	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)

	// Test data
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}

	t.Run("should create user successfully", func(t *testing.T) {
		ctx := context.Background()

		// Execute
		err := repo.Create(ctx, testUser)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, testUser.ID)
		assert.NotZero(t, testUser.CreatedAt)
		assert.NotZero(t, testUser.UpdatedAt)
	})

	t.Run("should fail on duplicate email", func(t *testing.T) {
		ctx := context.Background()

		// Create first user
		firstUser := &user.User{
			Name:     "Jane Doe",
			Email:    "jane@example.com",
			Password: "hashedpassword123",
		}
		err := repo.Create(ctx, firstUser)
		require.NoError(t, err)

		// Try to create second user with same email
		duplicateUser := &user.User{
			Name:     "Jane Smith",
			Email:    "jane@example.com", // Same email
			Password: "hashedpassword456",
		}
		err = repo.Create(ctx, duplicateUser)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email already exists")
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)
	ctx := context.Background()

	// Create test user
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}
	err := repo.Create(ctx, testUser)
	require.NoError(t, err)

	t.Run("should get user by ID", func(t *testing.T) {
		// Execute
		foundUser, err := repo.GetByID(ctx, testUser.ID)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testUser.ID, foundUser.ID)
		assert.Equal(t, testUser.Name, foundUser.Name)
		assert.Equal(t, testUser.Email, foundUser.Email)
		assert.Equal(t, testUser.Password, foundUser.Password)
	})

}

func TestUserRepository_GetByEmail(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)
	ctx := context.Background()

	// Create test user
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}
	err := repo.Create(ctx, testUser)
	require.NoError(t, err)

	t.Run("should get user by email", func(t *testing.T) {
		// Execute
		foundUser, err := repo.GetByEmail(ctx, testUser.Email)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, testUser.ID, foundUser.ID)
		assert.Equal(t, testUser.Email, foundUser.Email)
	})

	t.Run("should return error for non-existent email", func(t *testing.T) {
		// Execute
		_, err := repo.GetByEmail(ctx, "nonexistent@example.com")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})
}

func TestUserRepository_Update(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)
	ctx := context.Background()

	// Create test user
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}
	err := repo.Create(ctx, testUser)
	require.NoError(t, err)

	t.Run("should update user", func(t *testing.T) {
		// Update user data
		testUser.Name = "John Updated"
		testUser.Email = "john.updated@example.com"

		// Execute
		err := repo.Update(ctx, testUser)

		// Assert
		require.NoError(t, err)

		// Verify update
		updatedUser, err := repo.GetByID(ctx, testUser.ID)
		require.NoError(t, err)
		assert.Equal(t, "John Updated", updatedUser.Name)
		assert.Equal(t, "john.updated@example.com", updatedUser.Email)
	})
}

func TestUserRepository_Delete(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)
	ctx := context.Background()

	// Create test user
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}
	err := repo.Create(ctx, testUser)
	require.NoError(t, err)

	t.Run("should delete user", func(t *testing.T) {
		// Execute
		err := repo.Delete(ctx, testUser.ID)

		// Assert
		require.NoError(t, err)

		// Verify deletion
		_, err = repo.GetByID(ctx, testUser.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})
}

func TestUserRepository_EmailExists(t *testing.T) {
	testDB := setupTestDB(t)
	defer testDB.cleanup()

	queries := sqlc.New(testDB.db)
	repo := NewUserRepository(queries)
	ctx := context.Background()

	// Create test user
	testUser := &user.User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "hashedpassword123",
	}
	err := repo.Create(ctx, testUser)
	require.NoError(t, err)

	t.Run("should return true for existing email", func(t *testing.T) {
		// Execute
		exists, err := repo.EmailExists(ctx, testUser.Email)

		// Assert
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should return false for non-existing email", func(t *testing.T) {
		// Execute
		exists, err := repo.EmailExists(ctx, "nonexistent@example.com")

		// Assert
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
