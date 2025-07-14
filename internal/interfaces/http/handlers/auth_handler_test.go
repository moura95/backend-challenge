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
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
	"github.com/moura95/backend-challenge/internal/interfaces/http/ginx"
)

type authHandlerTestServer struct {
	container *postgres.PostgresContainer
	db        *sqlx.DB
	repos     *adapters.Repositories
	router    *gin.Engine
	handler   *AuthHandler
	cleanup   func()
}

func setupAuthHandlerTest(t *testing.T) *authHandlerTestServer {
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
	err = runAuthHandlerMigrations(db)
	require.NoError(t, err)

	// Setup repositories
	repos := adapters.NewRepositories(db)

	// Setup JWT token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012")
	require.NoError(t, err)

	// Setup use cases
	signUpUC := authUC.NewSignUpUseCase(repos.User, repos.Email, tokenMaker, nil)
	signInUC := authUC.NewSignInUseCase(repos.User, tokenMaker)
	verifyTokenUC := authUC.NewVerifyTokenUseCase(repos.User, tokenMaker)

	// Setup handler
	handler := NewAuthHandler(signUpUC, signInUC, verifyTokenUC)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup routes
	auth := router.Group("/auth")
	{
		auth.POST("/signup", handler.SignUp)
		auth.POST("/signin", handler.SignIn)
	}

	cleanup := func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	}

	return &authHandlerTestServer{
		container: postgresContainer,
		db:        db,
		repos:     repos,
		router:    router,
		handler:   handler,
		cleanup:   cleanup,
	}
}

func runAuthHandlerMigrations(db *sqlx.DB) error {
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

func TestAuthHandler_SignUp(t *testing.T) {
	server := setupAuthHandlerTest(t)
	defer server.cleanup()

	t.Run("should signup successfully with valid data", func(t *testing.T) {
		// Prepare request
		signupRequest := authUC.SignUpRequest{
			Name:     "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		// Make HTTP request
		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusCreated, recorder.Code)

		// Parse response
		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)
		assert.NotNil(t, response.Data)

		// Parse auth response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var authResponse AuthResponse
		err = json.Unmarshal(responseData, &authResponse)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", authResponse.User.Name)
		assert.Equal(t, "john@example.com", authResponse.User.Email)
		assert.NotEmpty(t, authResponse.User.ID)
		assert.Empty(t, authResponse.Token) // No token in signup response

		// Verify user was created in database
		var userCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, userCount)

		// Verify email was created in database
		var emailCount int
		err = server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE to_email = $1", "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, emailCount)
	})

	t.Run("should fail with invalid JSON", func(t *testing.T) {
		// Send invalid JSON
		req := httptest.NewRequest("POST", "/auth/signup", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusBadRequest, recorder.Code)

		var response ginx.Response
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid request format")
	})

	t.Run("should fail with duplicate email", func(t *testing.T) {
		// First signup
		firstRequest := authUC.SignUpRequest{
			Name:     "First User",
			Email:    "duplicate@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(firstRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)
		assert.Equal(t, http.StatusCreated, recorder.Code)

		// Second signup with same email
		secondRequest := authUC.SignUpRequest{
			Name:     "Second User",
			Email:    "duplicate@example.com", // Same email
			Password: "password456",
		}

		requestBody, err = json.Marshal(secondRequest)
		require.NoError(t, err)

		req = httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder = httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusConflict, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "email already exists")
	})

	t.Run("should fail with invalid email format", func(t *testing.T) {
		signupRequest := authUC.SignUpRequest{
			Name:     "Invalid Email",
			Email:    "invalid-email-format",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusBadRequest, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid")
	})

	t.Run("should fail with weak password", func(t *testing.T) {
		signupRequest := authUC.SignUpRequest{
			Name:     "Weak Password",
			Email:    "weak@example.com",
			Password: "123", // Too short
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusBadRequest, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "handler: signup failed: invalid request format")
	})
}

func TestAuthHandler_SignIn(t *testing.T) {
	server := setupAuthHandlerTest(t)
	defer server.cleanup()

	// Helper function to create a user
	createUser := func(name, email, password string) {
		signupRequest := authUC.SignUpRequest{
			Name:     name,
			Email:    email,
			Password: password,
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusCreated, recorder.Code)
	}

	t.Run("should signin successfully with valid credentials", func(t *testing.T) {
		// Create user first
		createUser("John Doe", "signin@example.com", "password123")

		// Signin request
		signinRequest := authUC.SignInRequest{
			Email:    "signin@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signinRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusOK, recorder.Code)

		// Parse response
		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Error)
		assert.NotNil(t, response.Data)

		// Parse auth response
		responseData, err := json.Marshal(response.Data)
		require.NoError(t, err)

		var authResponse AuthResponse
		err = json.Unmarshal(responseData, &authResponse)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", authResponse.User.Name)
		assert.Equal(t, "signin@example.com", authResponse.User.Email)
		assert.NotEmpty(t, authResponse.User.ID)
		assert.NotEmpty(t, authResponse.Token) // Token should be present in signin
	})

	t.Run("should fail with invalid email", func(t *testing.T) {
		signinRequest := authUC.SignInRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signinRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid credentials")
	})

	t.Run("should fail with wrong password", func(t *testing.T) {
		// Create user first
		createUser("Wrong Pass", "wrongpass@example.com", "correctpassword")

		signinRequest := authUC.SignInRequest{
			Email:    "wrongpass@example.com",
			Password: "wrongpassword",
		}

		requestBody, err := json.Marshal(signinRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "invalid credentials")
	})

	t.Run("should fail with empty email", func(t *testing.T) {
		signinRequest := authUC.SignInRequest{
			Email:    "",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signinRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "email is required")
	})

	t.Run("should fail with empty password", func(t *testing.T) {
		signinRequest := authUC.SignInRequest{
			Email:    "test@example.com",
			Password: "",
		}

		requestBody, err := json.Marshal(signinRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Assert HTTP response
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)

		var response ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Error)
		assert.Contains(t, response.Error, "password is required")
	})
}

func TestAuthHandler_ErrorMapping(t *testing.T) {
	t.Run("should map errors correctly", func(t *testing.T) {
		testCases := []struct {
			errorMessage   string
			expectedStatus int
			description    string
		}{
			{"email already exists", http.StatusConflict, "duplicate email"},
			{"invalid credentials", http.StatusUnauthorized, "auth failure"},
			{"user not found", http.StatusUnauthorized, "user missing"},
			{"email is required", http.StatusUnauthorized, "missing email"},
			{"password is required", http.StatusUnauthorized, "missing password"},
			{"invalid email format", http.StatusBadRequest, "bad format"},
			{"name is required", http.StatusBadRequest, "validation error"},
			{"some other error", http.StatusInternalServerError, "generic error"},
		}

		for _, tc := range testCases {
			err := fmt.Errorf("%s", tc.errorMessage)
			statusCode := getStatusCodeFromError(err)
			assert.Equal(t, tc.expectedStatus, statusCode,
				"Error '%s' should map to status %d", tc.errorMessage, tc.expectedStatus)
		}
	})
}

func TestAuthHandler_Integration_CompleteFlow(t *testing.T) {
	server := setupAuthHandlerTest(t)
	defer server.cleanup()

	t.Run("complete auth flow: signup → signin → verify", func(t *testing.T) {
		// 1. Signup
		signupRequest := authUC.SignUpRequest{
			Name:     "Complete Flow",
			Email:    "complete@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusCreated, recorder.Code)

		// 2. Signin
		signinRequest := authUC.SignInRequest{
			Email:    "complete@example.com",
			Password: "password123",
		}

		requestBody, err = json.Marshal(signinRequest)
		require.NoError(t, err)

		req = httptest.NewRequest("POST", "/auth/signin", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		recorder = httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)

		// Parse signin response to get token
		var signinResponse ginx.Response
		err = json.Unmarshal(recorder.Body.Bytes(), &signinResponse)
		require.NoError(t, err)

		responseData, err := json.Marshal(signinResponse.Data)
		require.NoError(t, err)

		var authResponse AuthResponse
		err = json.Unmarshal(responseData, &authResponse)
		require.NoError(t, err)

		token := authResponse.Token
		assert.NotEmpty(t, token)

		// 3. Verify token using handler method
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)

		user, err := server.handler.VerifyToken(c, token)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "Complete Flow", user.Name)
		assert.Equal(t, "complete@example.com", user.Email)

		// Verify all data persisted correctly
		var userCount, emailCount int
		err = server.db.Get(&userCount, "SELECT COUNT(*) FROM users WHERE email = $1", "complete@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, userCount)

		err = server.db.Get(&emailCount, "SELECT COUNT(*) FROM emails WHERE to_email = $1", "complete@example.com")
		require.NoError(t, err)
		assert.Equal(t, 1, emailCount)
	})
}

func TestAuthHandler_ContentType(t *testing.T) {
	server := setupAuthHandlerTest(t)
	defer server.cleanup()

	t.Run("should handle missing content-type", func(t *testing.T) {
		signupRequest := authUC.SignUpRequest{
			Name:     "No Content Type",
			Email:    "nocontenttype@example.com",
			Password: "password123",
		}

		requestBody, err := json.Marshal(signupRequest)
		require.NoError(t, err)

		// Don't set Content-Type header
		req := httptest.NewRequest("POST", "/auth/signup", bytes.NewBuffer(requestBody))
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Should still work (Gin is flexible)
		assert.Equal(t, http.StatusCreated, recorder.Code)
	})

	t.Run("should handle wrong content-type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/signup", strings.NewReader("not json"))
		req.Header.Set("Content-Type", "text/plain")
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		// Should fail with bad request
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestAuthHandler_HTTPMethods(t *testing.T) {
	server := setupAuthHandlerTest(t)
	defer server.cleanup()

	t.Run("should reject GET on signup endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/auth/signup", nil)
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("should reject PUT on signin endpoint", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/auth/signin", nil)
		recorder := httptest.NewRecorder()

		server.router.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}
