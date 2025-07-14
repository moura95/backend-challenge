package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	userUC "github.com/moura95/backend-challenge/internal/application/usecases/user"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
	"github.com/moura95/backend-challenge/internal/interfaces/http/ginx"
	"github.com/moura95/backend-challenge/internal/interfaces/http/middlewares"
)

type userHandlerTestServer struct {
	container   *postgres.PostgresContainer
	db          *sqlx.DB
	repos       *adapters.Repositories
	router      *gin.Engine
	userHandler *UserHandler
	authHandler *AuthHandler
	tokenMaker  jwt.Maker
	cleanup     func()
}

func setupUserHandlerTest(t *testing.T) *userHandlerTestServer {
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
	err = runUserHandlerMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	// Setup JWT token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	// Setup auth use cases
	signUpUC := authUC.NewSignUpUseCase(repos.User, repos.Email, tokenMaker, nil)
	signInUC := authUC.NewSignInUseCase(repos.User, tokenMaker)
	verifyTokenUC := authUC.NewVerifyTokenUseCase(repos.User, tokenMaker)

	// Setup user use cases
	getUserProfileUC := userUC.NewGetUserProfileUseCase(repos.User)
	updateUserUC := userUC.NewUpdateUserUseCase(repos.User)
	deleteUserUC := userUC.NewDeleteUserUseCase(repos.User)
	listUsersUC := userUC.NewListUsersUseCase(repos.User)

	// Setup handlers
	authHandler := NewAuthHandler(signUpUC, signInUC, verifyTokenUC)
	userHandler := NewUserHandler(getUserProfileUC, updateUserUC, deleteUserUC, listUsersUC)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup routes
	api := router.Group("/api")
	{
		// Auth routes (for creating users and getting tokens)
		auth := api.Group("/auth")
		{
			auth.POST("/signup", authHandler.SignUp)
			auth.POST("/signin", authHandler.SignIn)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middlewares.AuthMiddleware(verifyTokenUC))
		{
			account := protected.Group("/account")
			{
				account.GET("/me", userHandler.GetProfile)
				account.PUT("/me", userHandler.UpdateProfile)
				account.DELETE("/me", userHandler.DeleteProfile)
			}

			protected.GET("/users", userHandler.ListUsers)
		}
	}

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &userHandlerTestServer{
		container:   postgresContainer,
		db:          db,
		repos:       repos,
		router:      router,
		userHandler: userHandler,
		authHandler: authHandler,
		tokenMaker:  tokenMaker,
		cleanup:     cleanup,
	}
}

func runUserHandlerMigrations(db *sqlx.DB) error {
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
	`

	_, err := db.Exec(migrationSQL)
	return err
}

// Helper function to create a user and get auth token
func createUserAndGetToken(t *testing.T, server *userHandlerTestServer, name, email, password string) (string, string) {
	// Add a small delay to avoid conflicts in concurrent tests
	time.Sleep(10 * time.Millisecond)

	// 1. Signup
	signupRequest := authUC.SignUpRequest{
		Name:     name,
		Email:    email,
		Password: password,
	}

	requestBody, err := json.Marshal(signupRequest)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/auth/signup", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, req)

	// Check if signup failed due to duplicate email
	if recorder.Code != http.StatusCreated {
		t.Logf("Signup failed with status %d for email %s: %s", recorder.Code, email, recorder.Body.String())
		require.Equal(t, http.StatusCreated, recorder.Code, "Signup should succeed")
	}

	// 2. Signin to get token
	signinRequest := authUC.SignInRequest{
		Email:    email,
		Password: password,
	}

	requestBody, err = json.Marshal(signinRequest)
	require.NoError(t, err)

	req = httptest.NewRequest("POST", "/api/auth/signin", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	// Parse response to get token and userID
	var signinResponse ginx.Response
	err = json.Unmarshal(recorder.Body.Bytes(), &signinResponse)
	require.NoError(t, err)

	responseData, err := json.Marshal(signinResponse.Data)
	require.NoError(t, err)

	var authResponse AuthResponse
	err = json.Unmarshal(responseData, &authResponse)
	require.NoError(t, err)

	return authResponse.Token, authResponse.User.ID
}

// Helper function to make authenticated request
func makeAuthenticatedRequest(t *testing.T, server *userHandlerTestServer, method, path, token string, body []byte) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func TestUserHandler_GetProfile(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("should get user profile successfully", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "John Doe", "john@example.com", "password123")

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)
		assert.NotNil(t, response.Data)

		// Parse user response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var userResponse map[string]interface{}
		err = json.Unmarshal(responseData, &userResponse)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", userResponse["name"])
		assert.Equal(t, "john@example.com", userResponse["email"])
		assert.NotEmpty(t, userResponse["id"])
		assert.NotEmpty(t, userResponse["created_at"])
	})

	t.Run("should fail without authentication", func(t *testing.T) {
		// Make request without token
		req := httptest.NewRequest("GET", "/api/account/me", nil)
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "authorization header not provided")
	})

	t.Run("should fail with invalid token", func(t *testing.T) {
		// Make request with invalid token
		req := httptest.NewRequest("GET", "/api/account/me", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid or expired token")
	})

	t.Run("should fail with expired token", func(t *testing.T) {
		// Create user first
		token, userID := createUserAndGetToken(t, server, "Expired User", "expired@example.com", "password123")

		// Create expired token
		userUUID, err := server.tokenMaker.VerifyToken(token)
		require.NoError(t, err)

		// Create a token that expires immediately
		userUID, _ := uuid.Parse(userUUID.UserUUID)
		expiredToken, _, err := server.tokenMaker.CreateToken(userUID, -1*time.Hour)
		require.NoError(t, err)

		// Try to access with expired token
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", expiredToken, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid or expired token")

		// Verify that userID is still valid (just the token expired)
		assert.NotEmpty(t, userID)
	})
}

func TestUserHandler_UpdateProfile(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("should update user name successfully", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Original Name", "update@example.com", "password123")

		// Update request
		updateReq := UpdateUserRequest{
			Name:  "Updated Name",
			Email: "", // Keep same email
		}

		requestBody, err := json.Marshal(updateReq)
		require.NoError(t, err)

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token, requestBody)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)

		// Parse user response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var userResponse map[string]interface{}
		err = json.Unmarshal(responseData, &userResponse)
		require.NoError(t, err)

		assert.Equal(t, "Updated Name", userResponse["name"])
		assert.Equal(t, "update@example.com", userResponse["email"]) // Email unchanged

		// Verify in database
		var dbName string
		err = server.db.Get(&dbName, "SELECT name FROM users WHERE email = $1", "update@example.com")
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", dbName)
	})

	t.Run("should update user email successfully", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Email Update", "oldemail@example.com", "password123")

		// Update request
		updateReq := UpdateUserRequest{
			Name:  "", // Keep same name
			Email: "newemail@example.com",
		}

		requestBody, err := json.Marshal(updateReq)
		require.NoError(t, err)

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token, requestBody)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)

		// Parse user response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var userResponse map[string]interface{}
		err = json.Unmarshal(responseData, &userResponse)
		require.NoError(t, err)

		assert.Equal(t, "Email Update", userResponse["name"]) // Name unchanged
		assert.Equal(t, "newemail@example.com", userResponse["email"])

		// Verify in database
		var dbEmail string
		err = server.db.Get(&dbEmail, "SELECT email FROM users WHERE name = $1", "Email Update")
		require.NoError(t, err)
		assert.Equal(t, "newemail@example.com", dbEmail)
	})

	t.Run("should fail with duplicate email", func(t *testing.T) {
		// Create two users
		token1, _ := createUserAndGetToken(t, server, "User 1", "user1@example.com", "password123")
		_, _ = createUserAndGetToken(t, server, "User 2", "user2@example.com", "password123")

		// Try to update user1 with user2's email
		updateReq := UpdateUserRequest{
			Name:  "User 1 Updated",
			Email: "user2@example.com", // Already exists
		}

		requestBody, err := json.Marshal(updateReq)
		require.NoError(t, err)

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token1, requestBody)

		// Assert HTTP response
		assert.Equal(t, http.StatusConflict, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "email already exists")
	})

	t.Run("should fail with invalid JSON", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Invalid JSON", "invalidjson@example.com", "password123")

		// Make authenticated request with invalid JSON
		recorder := makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token, []byte("invalid json"))

		// Assert HTTP response
		assert.Equal(t, http.StatusBadRequest, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid request format")
	})

	t.Run("should handle empty update gracefully", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Empty Update", "empty@example.com", "password123")

		// Empty update request
		updateReq := UpdateUserRequest{
			Name:  "",
			Email: "",
		}

		requestBody, err := json.Marshal(updateReq)
		require.NoError(t, err)

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token, requestBody)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)

		// Values should remain unchanged
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var userResponse map[string]interface{}
		err = json.Unmarshal(responseData, &userResponse)
		require.NoError(t, err)

		assert.Equal(t, "Empty Update", userResponse["name"])
		assert.Equal(t, "empty@example.com", userResponse["email"])
	})
}

func TestUserHandler_DeleteProfile(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("should delete user successfully", func(t *testing.T) {
		// Create user and get token
		token, userID := createUserAndGetToken(t, server, "Delete Me", "delete@example.com", "password123")

		// Verify user exists before deletion
		var userCount int
		err := server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "delete@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, userCount)

		// Make authenticated delete request
		recorder := makeAuthenticatedRequest(t, server, "DELETE", "/api/account/me", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		// Verify user was deleted
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "delete@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)

		// Verify userID was valid
		assert.NotEmpty(t, userID)
	})

	t.Run("should fail to delete with invalid token after deletion", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Delete Again", "deleteagain@example.com", "password123")

		// Delete user first time
		recorder := makeAuthenticatedRequest(t, server, "DELETE", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		// Try to delete again with same token
		recorder = makeAuthenticatedRequest(t, server, "DELETE", "/api/account/me", token, nil)

		// Should fail because user no longer exists
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid or expired token")
	})

	t.Run("should not affect other users when deleting", func(t *testing.T) {
		// Create multiple users
		token1, _ := createUserAndGetToken(t, server, "Keep Me 1", "keep1@example.com", "password123")
		token2, _ := createUserAndGetToken(t, server, "Delete Me", "deleteme@example.com", "password123")
		token3, _ := createUserAndGetToken(t, server, "Keep Me 2", "keep2@example.com", "password123")

		// Delete middle user
		recorder := makeAuthenticatedRequest(t, server, "DELETE", "/api/account/me", token2, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		// Verify other users still exist and can access their profiles
		recorder1 := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token1, nil)
		assert.Equal(t, http.StatusOK, recorder1.Code)

		recorder3 := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token3, nil)
		assert.Equal(t, http.StatusOK, recorder3.Code)

		// Verify deleted user count
		var deletedCount int
		err := server.db.Get(&deletedCount, "SELECT COUNT(*) FROM users WHERE email = $1", "deleteme@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, deletedCount)

		// Verify kept users count
		var keptCount int
		err = server.db.Get(&keptCount, "SELECT COUNT(*) FROM users WHERE email IN ($1, $2)", "keep1@example.com", "keep2@example.com")
		require.NoError(t, err)
		assert.Equal(t, 2, keptCount)
	})
}

func TestUserHandler_ListUsers(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	// Create multiple test users
	setupTestUsers := func() string {
		// Generate unique timestamp to avoid conflicts
		timestamp := time.Now().UnixNano()

		// Create main user to get token
		token, _ := createUserAndGetToken(t, server, "Main User", fmt.Sprintf("main%d@example.com", timestamp), "password123")

		// Create additional users for listing with unique emails
		_, _ = createUserAndGetToken(t, server, "Alice Johnson", fmt.Sprintf("alice%d@example.com", timestamp), "password123")
		_, _ = createUserAndGetToken(t, server, "Bob Smith", fmt.Sprintf("bob%d@example.com", timestamp), "password123")
		_, _ = createUserAndGetToken(t, server, "Charlie Brown", fmt.Sprintf("charlie%d@test.com", timestamp), "password123")

		return token
	}

	t.Run("should list users with default pagination", func(t *testing.T) {
		token := setupTestUsers()

		// Make authenticated request
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/users", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)

		// Parse list response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var listResponse ListUsersResponse
		err = json.Unmarshal(responseData, &listResponse)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(listResponse.Users), 4) // At least 4 users
		assert.GreaterOrEqual(t, listResponse.Total, 4)
		assert.Equal(t, 1, listResponse.Page)

		// Verify response structure
		for _, user := range listResponse.Users {
			assert.NotEmpty(t, user.ID)
			assert.NotEmpty(t, user.Name)
			assert.NotEmpty(t, user.Email)
			assert.NotEmpty(t, user.CreatedAt)
		}
	})

	t.Run("should list users with pagination", func(t *testing.T) {
		token := setupTestUsers()

		// Request with specific pagination
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/users?page=1&page_size=2", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var listResponse ListUsersResponse
		err = json.Unmarshal(responseData, &listResponse)
		require.NoError(t, err)

		assert.Equal(t, 2, len(listResponse.Users)) // Page size 2
		assert.Equal(t, 1, listResponse.Page)
	})

	t.Run("should search users by name", func(t *testing.T) {
		token := setupTestUsers()

		// Search for Alice
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/users?search=Alice", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var listResponse ListUsersResponse
		err = json.Unmarshal(responseData, &listResponse)
		require.NoError(t, err)

		// Should find Alice Johnson
		found := false
		for _, user := range listResponse.Users {
			if user.Name == "Alice Johnson" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find Alice Johnson")
	})

	t.Run("should search users by email", func(t *testing.T) {
		token := setupTestUsers()

		// Search for test.com domain
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/users?search=test.com", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var listResponse ListUsersResponse
		err = json.Unmarshal(responseData, &listResponse)
		require.NoError(t, err)

		// Should find at least one user with test.com
		foundTestUser := false
		for _, user := range listResponse.Users {
			if strings.Contains(user.Email, "test.com") {
				foundTestUser = true
				break
			}
		}
		assert.True(t, foundTestUser, "Should find at least one user with test.com domain")
	})

	t.Run("should return empty results for non-existent search", func(t *testing.T) {
		token := setupTestUsers()

		// Search for non-existent term
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/users?search=nonexistentuser12345", token, nil)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var listResponse ListUsersResponse
		err = json.Unmarshal(responseData, &listResponse)
		require.NoError(t, err)

		assert.Empty(t, listResponse.Users)
		assert.Equal(t, 0, listResponse.Total)
	})

	t.Run("should fail without authentication", func(t *testing.T) {
		// Make request without token
		req := httptest.NewRequest("GET", "/api/users", nil)
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "authorization header not provided")
	})
}

func TestUserHandler_Integration_CompleteFlow(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("complete user management flow", func(t *testing.T) {
		// 1. Create user via signup
		token, userID := createUserAndGetToken(t, server, "Integration User", "integration@example.com", "password123")
		assert.NotEmpty(t, token)
		assert.NotEmpty(t, userID)

		// 2. Get user profile
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		var profileResponse ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &profileResponse)
		require.NoError(t, err)

		profileData, err := json.Marshal(profileResponse.Data)
		require.NoError(t, err)

		var userProfile map[string]interface{}
		err = json.Unmarshal(profileData, &userProfile)
		require.NoError(t, err)

		assert.Equal(t, "Integration User", userProfile["name"])
		assert.Equal(t, "integration@example.com", userProfile["email"])
		assert.Equal(t, userID, userProfile["id"])

		// 3. Update user profile
		updateReq := UpdateUserRequest{
			Name:  "Updated Integration User",
			Email: "updated.integration@example.com",
		}

		requestBody, err := json.Marshal(updateReq)
		require.NoError(t, err)

		recorder = makeAuthenticatedRequest(t, server, "PUT", "/api/account/me", token, requestBody)
		assert.Equal(t, http.StatusOK, recorder.Code)

		var updateResponse ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &updateResponse)
		require.NoError(t, err)

		updateData, err := json.Marshal(updateResponse.Data)
		require.NoError(t, err)

		var updatedUser map[string]interface{}
		err = json.Unmarshal(updateData, &updatedUser)
		require.NoError(t, err)

		assert.Equal(t, "Updated Integration User", updatedUser["name"])
		assert.Equal(t, "updated.integration@example.com", updatedUser["email"])

		// 4. Verify update persisted by getting profile again
		recorder = makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		var verifyResponse ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &verifyResponse)
		require.NoError(t, err)

		verifyData, err := json.Marshal(verifyResponse.Data)
		require.NoError(t, err)

		var verifiedUser map[string]interface{}
		err = json.Unmarshal(verifyData, &verifiedUser)
		require.NoError(t, err)

		assert.Equal(t, "Updated Integration User", verifiedUser["name"])
		assert.Equal(t, "updated.integration@example.com", verifiedUser["email"])

		// 5. Create another user to test listing
		_, _ = createUserAndGetToken(t, server, "Another User", "another@example.com", "password123")

		// 6. List users
		recorder = makeAuthenticatedRequest(t, server, "GET", "/api/users", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		var listResponse ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &listResponse)
		require.NoError(t, err)

		listData, err := json.Marshal(listResponse.Data)
		require.NoError(t, err)

		var userList ListUsersResponse
		err = json.Unmarshal(listData, &userList)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(userList.Users), 2) // At least 2 users
		assert.GreaterOrEqual(t, userList.Total, 2)

		// Verify our updated user is in the list
		foundUpdatedUser := false
		for _, user := range userList.Users {
			if user.Email == "updated.integration@example.com" {
				foundUpdatedUser = true
				assert.Equal(t, "Updated Integration User", user.Name)
				break
			}
		}
		assert.True(t, foundUpdatedUser, "Updated user should be in the list")

		// 7. Search for specific user
		recorder = makeAuthenticatedRequest(t, server, "GET", "/api/users?search=Updated", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		var searchResponse ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &searchResponse)
		require.NoError(t, err)

		searchData, err := json.Marshal(searchResponse.Data)
		require.NoError(t, err)

		var searchResults ListUsersResponse
		err = json.Unmarshal(searchData, &searchResults)
		require.NoError(t, err)

		assert.Greater(t, len(searchResults.Users), 0)

		// Should find our updated user
		foundInSearch := false
		for _, user := range searchResults.Users {
			if user.Name == "Updated Integration User" {
				foundInSearch = true
				break
			}
		}
		assert.True(t, foundInSearch, "Should find updated user in search")

		// 8. Finally, delete the user
		recorder = makeAuthenticatedRequest(t, server, "DELETE", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusNoContent, recorder.Code)

		// 9. Verify user was deleted (token should no longer work)
		recorder = makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		// Verify user was removed from database
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "updated.integration@example.com")
		require.NoError(t, err)
		assert.Equal(t, 0, userCount)
	})
}

func TestUserHandler_ErrorHandling(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("should handle malformed authorization header", func(t *testing.T) {
		malformedHeaders := []string{
			"Bearer",      // Missing token
			"Basic token", // Wrong type
			"token",       // Missing Bearer
			"Bearer ",     // Empty token
		}

		for _, header := range malformedHeaders {
			req := httptest.NewRequest("GET", "/api/account/me", nil)
			req.Header.Set("Authorization", header)
			recorder := httptest.NewRecorder()

			server.router.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)

			var response ginx.Response
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.NotEmpty(t, response.Error)
		}
	})

	t.Run("should handle various HTTP methods correctly", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Method Test", "methods@example.com", "password123")

		testCases := []struct {
			method       string
			path         string
			expectedCode int
			description  string
		}{
			{"GET", "/api/account/me", http.StatusOK, "GET profile should work"},
			{"POST", "/api/account/me", http.StatusNotFound, "POST profile should not be allowed"},
			{"PUT", "/api/account/me", http.StatusBadRequest, "PUT profile without body should fail"},
			{"DELETE", "/api/account/me", http.StatusNoContent, "DELETE profile should work"},
			{"PATCH", "/api/account/me", http.StatusNotFound, "PATCH profile should not be allowed"},
		}

		for _, tc := range testCases {
			// Skip DELETE test if user already deleted
			if tc.method == "DELETE" {
				// Create a fresh user for delete test
				deleteToken, _ := createUserAndGetToken(t, server, "Delete Test", "deletetest@example.com", "password123")
				recorder := makeAuthenticatedRequest(t, server, tc.method, tc.path, deleteToken, nil)
				assert.Equal(t, tc.expectedCode, recorder.Code, tc.description)
			} else if tc.method != "DELETE" {
				recorder := makeAuthenticatedRequest(t, server, tc.method, tc.path, token, nil)
				assert.Equal(t, tc.expectedCode, recorder.Code, tc.description)
			}
		}
	})

	t.Run("should handle concurrent requests", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Concurrent User", "concurrent@example.com", "password123")

		// Make multiple concurrent requests
		done := make(chan bool, 5)
		errors := make(chan error, 5)

		for i := 0; i < 5; i++ {
			go func(id int) {
				recorder := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)
				if recorder.Code != http.StatusOK {
					errors <- fmt.Errorf("request %d failed with status %d", id, recorder.Code)
				}
				done <- true
			}(i)
		}

		// Wait for all requests
		for i := 0; i < 5; i++ {
			<-done
		}

		// Check for errors
		select {
		case err := <-errors:
			t.Errorf("Concurrent request failed: %v", err)
		default:
			// No errors, test passed
		}
	})

}

func TestUserHandler_Security(t *testing.T) {
	server := setupUserHandlerTest(t)
	defer server.cleanup()

	t.Run("should not allow user to access another user's data", func(t *testing.T) {
		// Create two users
		token1, userID1 := createUserAndGetToken(t, server, "User 1", "user1@security.com", "password123")
		token2, userID2 := createUserAndGetToken(t, server, "User 2", "user2@security.com", "password123")

		assert.NotEqual(t, userID1, userID2)

		// User 1 gets their own profile
		recorder1 := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token1, nil)
		assert.Equal(t, http.StatusOK, recorder1.Code)

		var response1 ginx.Response
		err := json.Unmarshal(recorder1.Body.Bytes(), &response1)
		require.NoError(t, err)

		profileData1, err := json.Marshal(response1.Data)
		require.NoError(t, err)

		var userProfile1 map[string]interface{}
		err = json.Unmarshal(profileData1, &userProfile1)
		require.NoError(t, err)

		// User 2 gets their own profile
		recorder2 := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token2, nil)
		assert.Equal(t, http.StatusOK, recorder2.Code)

		var response2 ginx.Response
		err = json.Unmarshal(recorder2.Body.Bytes(), &response2)
		require.NoError(t, err)

		profileData2, err := json.Marshal(response2.Data)
		require.NoError(t, err)

		var userProfile2 map[string]interface{}
		err = json.Unmarshal(profileData2, &userProfile2)
		require.NoError(t, err)

		// Verify they get different data
		assert.NotEqual(t, userProfile1["id"], userProfile2["id"])
		assert.NotEqual(t, userProfile1["email"], userProfile2["email"])
		assert.Equal(t, "User 1", userProfile1["name"])
		assert.Equal(t, "User 2", userProfile2["name"])
	})

	t.Run("should not expose password in any response", func(t *testing.T) {
		// Create user and get token
		token, _ := createUserAndGetToken(t, server, "Password Test", "password@example.com", "password123")

		// Get profile
		recorder := makeAuthenticatedRequest(t, server, "GET", "/api/account/me", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Verify password is not in response
		responseBody := recorder.Body.String()
		assert.NotContains(t, responseBody, "password123")
		assert.NotContains(t, responseBody, "$2a$") // bcrypt prefix

		// List users
		recorder = makeAuthenticatedRequest(t, server, "GET", "/api/users", token, nil)
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Verify passwords are not in list response
		responseBody = recorder.Body.String()
		assert.NotContains(t, responseBody, "password123")
		assert.NotContains(t, responseBody, "$2a$") // bcrypt prefix
	})
}
