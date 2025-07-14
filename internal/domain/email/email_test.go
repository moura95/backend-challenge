package email

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWelcomeEmail(t *testing.T) {
	t.Run("should create welcome email successfully with valid data", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "john@example.com",
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, email)
		assert.NotEmpty(t, email.ID)
		assert.Equal(t, "john@example.com", email.To)
		assert.Equal(t, "Welcome to Backend Challenge!", email.Subject)
		assert.Contains(t, email.Body, "John Doe")
		assert.Contains(t, email.Body, "Welcome to Backend Challenge")
		assert.Equal(t, EmailTypeWelcome, email.Type)
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 0, email.Attempts)
		assert.Equal(t, 3, email.MaxAttempts)
		assert.NotZero(t, email.CreatedAt)
		assert.Nil(t, email.SentAt)
		assert.Empty(t, email.ErrorMsg)
	})

	t.Run("should fail with empty user name", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "", // Empty name
			UserEmail: "john@example.com",
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, email)
		assert.Contains(t, err.Error(), "user name is required")
	})

	t.Run("should fail with empty user email", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "", // Empty email
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, email)
		fmt.Println(err.Error())
		assert.Contains(t, err.Error(), "user email validation failed: email is required")
	})

	t.Run("should fail with invalid email format", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "John Doe",
			UserEmail: "invalid-email", // Invalid format
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, email)
		assert.Contains(t, err.Error(), "invalid email format")
	})

	t.Run("should fail with empty user ID", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    "", // Empty ID
			UserName:  "John Doe",
			UserEmail: "john@example.com",
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, email)
		assert.Contains(t, err.Error(), "user ID is required")
	})

	t.Run("should handle special characters in user name", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "José María Ñoño-García",
			UserEmail: "jose@example.com",
		}

		// Act
		email, err := NewWelcomeEmail(data)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, email)
		assert.Contains(t, email.Body, "José María Ñoño-García")
	})

	t.Run("should generate unique IDs for different emails", func(t *testing.T) {
		// Arrange
		data1 := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "User 1",
			UserEmail: "user1@example.com",
		}
		data2 := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "User 2",
			UserEmail: "user2@example.com",
		}

		// Act
		email1, err1 := NewWelcomeEmail(data1)
		email2, err2 := NewWelcomeEmail(data2)

		// Assert
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, email1.ID, email2.ID)
	})
}

func TestEmail_MarkAsSent(t *testing.T) {
	t.Run("should mark email as sent with timestamp", func(t *testing.T) {
		// Arrange
		email := &Email{
			ID:          uuid.New(),
			Status:      StatusPending,
			Attempts:    0,
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
		}
		beforeTime := time.Now()

		// Act
		email.MarkAsSent()

		// Assert
		assert.Equal(t, StatusSent, email.Status)
		assert.NotNil(t, email.SentAt)
		assert.True(t, email.SentAt.After(beforeTime) || email.SentAt.Equal(beforeTime))
		assert.True(t, email.SentAt.Before(time.Now().Add(time.Second)))
	})

	t.Run("should update sent timestamp on multiple calls", func(t *testing.T) {
		// Arrange
		email := &Email{
			ID:          uuid.New(),
			Status:      StatusPending,
			Attempts:    0,
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
		}

		// Act
		email.MarkAsSent()
		firstSentAt := *email.SentAt

		time.Sleep(10 * time.Millisecond) // Small delay
		email.MarkAsSent()
		secondSentAt := *email.SentAt

		// Assert
		assert.True(t, secondSentAt.After(firstSentAt))
	})
}

func TestEmail_MarkAsFailed(t *testing.T) {
	t.Run("should increment attempts and stay pending when under max attempts", func(t *testing.T) {
		// Arrange
		email := &Email{
			ID:          uuid.New(),
			Status:      StatusPending,
			Attempts:    0,
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
		}
		errorMsg := "SMTP connection failed"

		// Act
		email.MarkAsFailed(errorMsg)

		// Assert
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 1, email.Attempts)
		assert.Equal(t, errorMsg, email.ErrorMsg)
	})

	t.Run("should mark as failed when reaching max attempts", func(t *testing.T) {
		// Arrange
		email := &Email{
			ID:          uuid.New(),
			Status:      StatusPending,
			Attempts:    2, // Already 2 attempts
			MaxAttempts: 3,
			CreatedAt:   time.Now(),
		}
		errorMsg := "Final SMTP failure"

		// Act
		email.MarkAsFailed(errorMsg)

		// Assert
		assert.Equal(t, StatusFailed, email.Status)
		assert.Equal(t, 3, email.Attempts)
		assert.Equal(t, errorMsg, email.ErrorMsg)
	})

}

func TestGenerateWelcomeEmailBody(t *testing.T) {
	t.Run("should generate HTML email body with user name", func(t *testing.T) {
		// Arrange
		userName := "John Doe"

		// Act
		body := generateWelcomeEmailBody(userName)

		// Assert
		assert.Contains(t, body, "<!DOCTYPE html>")
		assert.Contains(t, body, "<html>")
		assert.Contains(t, body, "<body>")
		assert.Contains(t, body, "Welcome to Backend Challenge, John Doe!")
		assert.Contains(t, body, "Thank you for signing up!")
		assert.Contains(t, body, "The Backend Challenge Team")
		assert.Contains(t, body, "</html>")
	})

	t.Run("should handle special characters in user name", func(t *testing.T) {
		// Arrange
		userName := "José María & Co."

		// Act
		body := generateWelcomeEmailBody(userName)

		// Assert
		assert.Contains(t, body, "José María & Co.")
	})

	t.Run("should handle empty user name", func(t *testing.T) {
		// Arrange
		userName := ""

		// Act
		body := generateWelcomeEmailBody(userName)

		// Assert
		assert.Contains(t, body, "Welcome to Backend Challenge, !")
		assert.Contains(t, body, "<!DOCTYPE html>")
	})

	t.Run("should generate valid HTML structure", func(t *testing.T) {
		// Arrange
		userName := "Test User"

		// Act
		body := generateWelcomeEmailBody(userName)

		// Assert
		assert.Contains(t, body, "<title>Welcome!</title>")
		assert.Contains(t, body, "<h1>")
		assert.Contains(t, body, "<p>")
		assert.Contains(t, body, "<br>")
		// Verify proper HTML structure
		assert.True(t, len(body) > 100) // Should be substantial HTML
	})
}

func TestEmailTypes_Constants(t *testing.T) {
	t.Run("should have correct email type constants", func(t *testing.T) {
		assert.Equal(t, EmailType("welcome"), EmailTypeWelcome)
	})
}

func TestEmailStatus_Constants(t *testing.T) {
	t.Run("should have correct status constants", func(t *testing.T) {
		assert.Equal(t, Status("pending"), StatusPending)
		assert.Equal(t, Status("sent"), StatusSent)
		assert.Equal(t, Status("failed"), StatusFailed)
	})
}

func TestWelcomeEmailData_Struct(t *testing.T) {
	t.Run("should create welcome email data struct", func(t *testing.T) {
		// Arrange & Act
		data := WelcomeEmailData{
			UserID:    "123e4567-e89b-12d3-a456-426614174000",
			UserName:  "Test User",
			UserEmail: "test@example.com",
		}

		// Assert
		assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", data.UserID)
		assert.Equal(t, "Test User", data.UserName)
		assert.Equal(t, "test@example.com", data.UserEmail)
	})
}

func TestEmail_CompleteWorkflow(t *testing.T) {
	t.Run("should handle complete email lifecycle", func(t *testing.T) {
		// Arrange - Create email
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "Workflow User",
			UserEmail: "workflow@example.com",
		}

		email, err := NewWelcomeEmail(data)
		require.NoError(t, err)

		// Assert initial state
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 0, email.Attempts)
		assert.True(t, email.CanRetry())

		// Act - First failure
		email.MarkAsFailed("First attempt failed")
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 1, email.Attempts)
		assert.True(t, email.CanRetry())

		// Act - Second failure
		email.MarkAsFailed("Second attempt failed")
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 2, email.Attempts)
		assert.True(t, email.CanRetry())

		// Act - Third failure (final)
		email.MarkAsFailed("Final attempt failed")
		assert.Equal(t, StatusFailed, email.Status)
		assert.Equal(t, 3, email.Attempts)
		assert.False(t, email.CanRetry())
	})

	t.Run("should handle successful send after failures", func(t *testing.T) {
		// Arrange
		data := WelcomeEmailData{
			UserID:    uuid.New().String(),
			UserName:  "Success User",
			UserEmail: "success@example.com",
		}

		email, err := NewWelcomeEmail(data)
		require.NoError(t, err)

		// Act - One failure then success
		email.MarkAsFailed("Temporary failure")
		assert.Equal(t, StatusPending, email.Status)
		assert.Equal(t, 1, email.Attempts)

		email.MarkAsSent()
		assert.Equal(t, StatusSent, email.Status)
		assert.NotNil(t, email.SentAt)
		assert.False(t, email.CanRetry()) // Can't retry sent emails
	})
}
