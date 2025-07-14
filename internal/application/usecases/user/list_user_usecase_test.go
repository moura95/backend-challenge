package user

import (
	"context"
	"fmt"
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
)

type listUsersTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	cleanup   func()
}

func setupListUsersTest(t *testing.T) *listUsersTestServer {
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
	err = runListUsersMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &listUsersTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		cleanup:   cleanup,
	}
}

func runListUsersMigrations(db *sqlx.DB) error {
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
	CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

// Helper function to create multiple test users
func createTestUsersForList(t *testing.T, server *listUsersTestServer) []*user.User {
	ctx := context.Background()

	users := []*user.User{}

	// Create diverse test data
	testData := []struct {
		name     string
		email    string
		password string
	}{
		{"Alice Johnson", "alice@example.com", "password123"},
		{"Bob Smith", "bob@example.com", "password123"},
		{"Charlie Brown", "charlie@test.com", "password123"},
		{"Diana Prince", "diana@example.com", "password123"},
		{"Eve Adams", "eve@test.com", "password123"},
		{"Frank Miller", "frank@example.com", "password123"},
		{"Grace Lee", "grace@test.com", "password123"},
		{"Henry Wilson", "henry@example.com", "password123"},
		{"Ivy Chen", "ivy@test.com", "password123"},
		{"Jack Taylor", "jack@example.com", "password123"},
	}

	for _, data := range testData {
		testUser, err := user.NewUser(data.name, data.email, data.password)
		require.NoError(t, err)

		err = server.repos.User.Create(ctx, testUser)
		require.NoError(t, err)

		users = append(users, testUser)
	}

	return users
}

func TestListUsersUseCase_Execute(t *testing.T) {
	server := setupListUsersTest(t)
	defer server.cleanup()

	ctx := context.Background()

	t.Run("should list users with default pagination", func(t *testing.T) {
		// Create test users
		testUsers := createTestUsersForList(t, server)

		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Execute with default values
		req := ListUsersRequest{}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result.Users), 10) // Should have at least 10 users (default page size)
		assert.GreaterOrEqual(t, result.Total, len(testUsers))
		assert.Equal(t, 1, result.Page) // Default page
	})

	t.Run("should handle pagination correctly", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// First page
		req1 := ListUsersRequest{Page: 1, PageSize: 5}
		result1, err := useCase.Execute(ctx, req1)
		require.NoError(t, err)
		assert.Len(t, result1.Users, 5)
		assert.Equal(t, 1, result1.Page)

		// Second page
		req2 := ListUsersRequest{Page: 2, PageSize: 5}
		result2, err := useCase.Execute(ctx, req2)
		require.NoError(t, err)
		assert.Len(t, result2.Users, 5)
		assert.Equal(t, 2, result2.Page)

		// Verify different users in each page
		firstPageIDs := make(map[string]bool)
		for _, u := range result1.Users {
			firstPageIDs[u.ID.String()] = true
		}

		for _, u := range result2.Users {
			assert.False(t, firstPageIDs[u.ID.String()], "User should not appear in both pages")
		}
	})

	t.Run("should search users by name", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Search for "Alice"
		req := ListUsersRequest{Search: "Alice"}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Should find Alice Johnson
		found := false
		for _, u := range result.Users {
			if u.Name == "Alice Johnson" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find Alice Johnson")
	})

	t.Run("should search users by email", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Search for "test.com" domain
		req := ListUsersRequest{Search: "test.com"}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)

		// All returned users should have test.com in email
		for _, u := range result.Users {
			assert.Contains(t, u.Email, "test.com")
		}
	})

	t.Run("should handle case insensitive search", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Search with different cases
		searches := []string{"alice", "ALICE", "Alice", "aLiCe"}

		for _, search := range searches {
			req := ListUsersRequest{Search: search}
			result, err := useCase.Execute(ctx, req)
			require.NoError(t, err)

			// Should find at least one user (Alice Johnson)
			assert.Greater(t, len(result.Users), 0, fmt.Sprintf("Search '%s' should return results", search))
		}
	})

	t.Run("should return empty results for non-existent search", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Search for non-existent term
		req := ListUsersRequest{Search: "nonexistentuser12345"}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Users)
		assert.Equal(t, 0, result.Total)
	})

	t.Run("should handle invalid page numbers", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Test with invalid page numbers
		invalidPages := []int{-1, 0}

		for _, page := range invalidPages {
			req := ListUsersRequest{Page: page, PageSize: 10}
			result, err := useCase.Execute(ctx, req)
			require.NoError(t, err)
			assert.Equal(t, 1, result.Page, "Should default to page 1")
		}
	})

	t.Run("should handle invalid page sizes", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Test with page size too small
		req1 := ListUsersRequest{Page: 1, PageSize: -1}
		result1, err := useCase.Execute(ctx, req1)
		require.NoError(t, err)
		assert.Len(t, result1.Users, 10) // Should default to 10

		// Test with page size too large
		req2 := ListUsersRequest{Page: 1, PageSize: 150}
		result2, err := useCase.Execute(ctx, req2)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result2.Users), 100) // Should cap at 100
	})

	t.Run("should handle empty database", func(t *testing.T) {
		// Create fresh test server
		freshServer := setupListUsersTest(t)
		defer freshServer.cleanup()

		// Create use case
		useCase := NewListUsersUseCase(freshServer.repos.User)

		// Execute
		req := ListUsersRequest{Page: 1, PageSize: 10}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Users)
		assert.Equal(t, 0, result.Total)
		assert.Equal(t, 1, result.Page)
	})

	t.Run("should handle large page numbers", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Request page that's beyond available data
		req := ListUsersRequest{Page: 1000, PageSize: 10}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Users)
		assert.Equal(t, 1000, result.Page)
	})

	t.Run("should maintain consistent ordering", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Execute same request multiple times
		req := ListUsersRequest{Page: 1, PageSize: 5}

		results := make([]ListUsersResponse, 3)
		for i := 0; i < 3; i++ {
			result, err := useCase.Execute(ctx, req)
			require.NoError(t, err)
			results[i] = *result
		}

		// Verify ordering is consistent
		for i := 1; i < len(results); i++ {
			assert.Equal(t, len(results[0].Users), len(results[i].Users))
			for j := 0; j < len(results[0].Users); j++ {
				assert.Equal(t, results[0].Users[j].ID, results[i].Users[j].ID,
					"User order should be consistent across requests")
			}
		}
	})

	t.Run("should handle special characters in search", func(t *testing.T) {
		// Create user with special characters
		specialUser, err := user.NewUser("José María Ñoño", "jose@example.com", "password123")
		require.NoError(t, err)
		err = server.repos.User.Create(ctx, specialUser)
		require.NoError(t, err)

		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Search for special characters
		req := ListUsersRequest{Search: "José"}
		result, err := useCase.Execute(ctx, req)

		// Assert
		require.NoError(t, err)

		// Should find the user with special characters
		found := false
		for _, u := range result.Users {
			if u.Name == "José María Ñoño" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find user with special characters")
	})

	t.Run("should handle SQL injection attempts", func(t *testing.T) {
		// Create use case
		useCase := NewListUsersUseCase(server.repos.User)

		// Try SQL injection in search
		maliciousSearches := []string{
			"'; DROP TABLE users; --",
			"' OR '1'='1",
			"admin'--",
			"' UNION SELECT * FROM users --",
		}

		for _, search := range maliciousSearches {
			req := ListUsersRequest{Search: search}
			result, err := useCase.Execute(ctx, req)

			// Should not error and should not return unexpected results
			require.NoError(t, err)
			assert.NotNil(t, result)
			// Result should be empty or normal users, not injection results
		}
	})
}
