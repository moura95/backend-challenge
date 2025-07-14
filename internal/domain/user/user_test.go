package user

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	t.Run("should create user successfully with valid data", func(t *testing.T) {
		// Arrange
		name := "John Doe"
		email := "john@example.com"
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.NotEmpty(t, user.ID)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, email, user.Email)
		assert.NotEqual(t, password, user.Password) // Should be hashed
		assert.NotEmpty(t, user.Password)
		assert.NotZero(t, user.CreatedAt)
		assert.NotZero(t, user.UpdatedAt)
	})

	t.Run("should fail with invalid email format", func(t *testing.T) {
		// Arrange
		name := "John Doe"
		email := "invalid-email"
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid email format")
	})

	t.Run("should fail with empty email", func(t *testing.T) {
		// Arrange
		name := "John Doe"
		email := ""
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid email format")
	})

	t.Run("should fail with short name", func(t *testing.T) {
		// Arrange
		name := "J" // Too short
		email := "john@example.com"
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "name must be at least 2 characters")
	})

	t.Run("should fail with long name", func(t *testing.T) {
		// Arrange
		name := strings.Repeat("A", 101) // Too long (>100 chars)
		email := "john@example.com"
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "name must be less than 100 characters")
	})

	t.Run("should fail with weak password", func(t *testing.T) {
		// Arrange
		name := "John Doe"
		email := "john@example.com"
		password := "123" // Too short

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "password must be at least 6 characters")
	})

	t.Run("should handle special characters in name", func(t *testing.T) {
		// Arrange
		name := "José María Ñoño-García O'Connor"
		email := "jose@example.com"
		password := "password123"

		// Act
		user, err := NewUser(name, email, password)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, name, user.Name)
	})

	t.Run("should generate unique IDs for different users", func(t *testing.T) {
		// Arrange & Act
		user1, err1 := NewUser("User 1", "user1@example.com", "password123")
		user2, err2 := NewUser("User 2", "user2@example.com", "password123")

		// Assert
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, user1.ID, user2.ID)
	})

	t.Run("should hash password correctly", func(t *testing.T) {
		// Arrange
		password := "mySecretPassword123"

		// Act
		user, err := NewUser("John Doe", "john@example.com", password)

		// Assert
		require.NoError(t, err)
		assert.NotEqual(t, password, user.Password)
		assert.True(t, len(user.Password) > len(password)) // Hashed should be longer

		// Should be able to check password
		err = user.CheckPassword(password)
		assert.NoError(t, err)
	})

}

func TestUser_UpdateUser(t *testing.T) {
	// Helper function to create a test user
	createTestUser := func() *User {
		user, err := NewUser("John Doe", "john@example.com", "password123")
		require.NoError(t, err)
		return user
	}

	t.Run("should update name successfully", func(t *testing.T) {
		// Arrange
		user := createTestUser()
		originalEmail := user.Email
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(1 * time.Millisecond) // Ensure time difference

		// Act
		err := user.UpdateUser("John Updated", "")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "John Updated", user.Name)
		assert.Equal(t, originalEmail, user.Email) // Email should remain unchanged
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("should update email successfully", func(t *testing.T) {
		// Arrange
		user := createTestUser()
		originalName := user.Name
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		// Act
		err := user.UpdateUser("", "john.updated@example.com")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, originalName, user.Name) // Name should remain unchanged
		assert.Equal(t, "john.updated@example.com", user.Email)
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("should update both name and email successfully", func(t *testing.T) {
		// Arrange
		user := createTestUser()
		originalUpdatedAt := user.UpdatedAt
		time.Sleep(1 * time.Millisecond)

		// Act
		err := user.UpdateUser("John Both", "john.both@example.com")

		// Assert
		assert.NoError(t, err)
		assert.Equal(t, "John Both", user.Name)
		assert.Equal(t, "john.both@example.com", user.Email)
		assert.True(t, user.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("should fail with invalid name", func(t *testing.T) {
		// Arrange
		user := createTestUser()
		originalName := user.Name
		originalUpdatedAt := user.UpdatedAt

		// Act
		err := user.UpdateUser("J", "") // Too short

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name must be at least 2 characters")
		assert.Equal(t, originalName, user.Name)           // Should remain unchanged
		assert.Equal(t, originalUpdatedAt, user.UpdatedAt) // Should not update timestamp on error
	})

	t.Run("should fail with invalid email", func(t *testing.T) {
		// Arrange
		user := createTestUser()
		originalEmail := user.Email
		originalUpdatedAt := user.UpdatedAt

		// Act
		err := user.UpdateUser("", "invalid-email")

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email format")
		assert.Equal(t, originalEmail, user.Email)         // Should remain unchanged
		assert.Equal(t, originalUpdatedAt, user.UpdatedAt) // Should not update timestamp on error
	})

}

func TestUser_CheckPassword(t *testing.T) {
	t.Run("should verify correct password", func(t *testing.T) {
		// Arrange
		password := "mySecretPassword123"
		user, err := NewUser("John Doe", "john@example.com", password)
		require.NoError(t, err)

		// Act
		err = user.CheckPassword(password)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("should reject incorrect password", func(t *testing.T) {
		// Arrange
		password := "mySecretPassword123"
		user, err := NewUser("John Doe", "john@example.com", password)
		require.NoError(t, err)

		// Act
		err = user.CheckPassword("wrongPassword")

		// Assert
		assert.Error(t, err)
	})

	t.Run("should be case sensitive", func(t *testing.T) {
		// Arrange
		password := "MyPassword123"
		user, err := NewUser("John Doe", "john@example.com", password)
		require.NoError(t, err)

		// Act
		err = user.CheckPassword("mypassword123") // Different case

		// Assert
		assert.Error(t, err)
	})

	t.Run("should reject empty password", func(t *testing.T) {
		// Arrange
		password := "validPassword123"
		user, err := NewUser("John Doe", "john@example.com", password)
		require.NoError(t, err)

		// Act
		err = user.CheckPassword("")

		// Assert
		assert.Error(t, err)
	})
}

func TestUser_CompleteWorkflow(t *testing.T) {
	t.Run("should handle complete user lifecycle", func(t *testing.T) {
		// Arrange - Create user
		originalName := "John Doe"
		originalEmail := "john@example.com"
		password := "password123"

		// Act - Create
		user, err := NewUser(originalName, originalEmail, password)
		require.NoError(t, err)

		// Assert initial state
		assert.Equal(t, originalName, user.Name)
		assert.Equal(t, originalEmail, user.Email)
		assert.NoError(t, user.CheckPassword(password))

		// Act - Update name
		newName := "John Updated"
		err = user.UpdateUser(newName, "")
		require.NoError(t, err)
		assert.Equal(t, newName, user.Name)
		assert.Equal(t, originalEmail, user.Email)

		// Act - Update email
		newEmail := "john.updated@example.com"
		err = user.UpdateUser("", newEmail)
		require.NoError(t, err)
		assert.Equal(t, newName, user.Name)
		assert.Equal(t, newEmail, user.Email)

		// Act - Update both
		finalName := "John Final"
		finalEmail := "john.final@example.com"
		err = user.UpdateUser(finalName, finalEmail)
		require.NoError(t, err)
		assert.Equal(t, finalName, user.Name)
		assert.Equal(t, finalEmail, user.Email)

		// Assert password still works
		assert.NoError(t, user.CheckPassword(password))

		// Act - Convert to response
		response := user.ToResponse()
		assert.Equal(t, user.ID.String(), response.ID)
		assert.Equal(t, finalName, response.Name)
		assert.Equal(t, finalEmail, response.Email)
	})
}
