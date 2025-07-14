package jwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasetoMaker(t *testing.T) {
	t.Run("should create maker with valid key", func(t *testing.T) {
		// Valid 32-character key
		validKey := "12345678901234567890123456789012"

		maker, err := NewPasetoMaker(validKey)

		require.NoError(t, err)
		assert.NotNil(t, maker)
		assert.IsType(t, &PasetoMaker{}, maker)
	})

	t.Run("should fail with short key", func(t *testing.T) {
		// Too short key
		shortKey := "short"

		maker, err := NewPasetoMaker(shortKey)

		assert.Error(t, err)
		assert.Nil(t, maker)
		assert.Contains(t, err.Error(), "invalid key size")
		assert.Contains(t, err.Error(), "32 characters")
	})

	t.Run("should fail with long key", func(t *testing.T) {
		// Too long key
		longKey := "123456789012345678901234567890123" // 33 characters

		maker, err := NewPasetoMaker(longKey)

		assert.Error(t, err)
		assert.Nil(t, maker)
		assert.Contains(t, err.Error(), "invalid key size")
	})

	t.Run("should fail with empty key", func(t *testing.T) {
		maker, err := NewPasetoMaker("")

		assert.Error(t, err)
		assert.Nil(t, maker)
		assert.Contains(t, err.Error(), "invalid key size")
	})

}

func TestPasetoMaker_CreateToken(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	maker, err := NewPasetoMaker(validKey)
	require.NoError(t, err)

	t.Run("should create token successfully", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Hour

		tokenString, payload, err := maker.CreateToken(userID, duration)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		assert.Equal(t, userID.String(), payload.UserUUID)
		assert.NotEmpty(t, payload.UUID)
		assert.True(t, payload.ExpiredAt.After(time.Now()))
		assert.True(t, payload.IssuedAt.Before(time.Now().Add(time.Second)))
	})

	t.Run("should create different tokens for same user", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Hour

		token1, payload1, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		token2, payload2, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		// Tokens should be different
		assert.NotEqual(t, token1, token2)
		assert.NotEqual(t, payload1.UUID, payload2.UUID)
		// But user should be same
		assert.Equal(t, payload1.UserUUID, payload2.UserUUID)
	})

	t.Run("should create tokens with different durations", func(t *testing.T) {
		userID := uuid.New()

		// Short duration
		shortDuration := 5 * time.Minute
		tokenShort, payloadShort, err := maker.CreateToken(userID, shortDuration)
		require.NoError(t, err)

		// Long duration
		longDuration := 24 * time.Hour
		tokenLong, payloadLong, err := maker.CreateToken(userID, longDuration)
		require.NoError(t, err)

		assert.NotEqual(t, tokenShort, tokenLong)
		assert.True(t, payloadLong.ExpiredAt.After(payloadShort.ExpiredAt))
	})

	t.Run("should handle zero duration", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Duration(0)

		tokenString, payload, err := maker.CreateToken(userID, duration)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		// Token should be expired immediately
		assert.True(t, payload.ExpiredAt.Before(time.Now()) || payload.ExpiredAt.Equal(payload.IssuedAt))
	})

	t.Run("should handle negative duration", func(t *testing.T) {
		userID := uuid.New()
		duration := -time.Hour

		tokenString, payload, err := maker.CreateToken(userID, duration)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		// Token should be expired
		assert.True(t, payload.ExpiredAt.Before(time.Now()))
	})
}

func TestPasetoMaker_VerifyToken(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	maker, err := NewPasetoMaker(validKey)
	require.NoError(t, err)

	t.Run("should verify valid token", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Hour

		// Create token
		tokenString, originalPayload, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		// Verify token
		verifiedPayload, err := maker.VerifyToken(tokenString)

		require.NoError(t, err)
		assert.NotNil(t, verifiedPayload)
		assert.Equal(t, originalPayload.UUID, verifiedPayload.UUID)
		assert.Equal(t, originalPayload.UserUUID, verifiedPayload.UserUUID)
		assert.Equal(t, originalPayload.IssuedAt.Unix(), verifiedPayload.IssuedAt.Unix())
		assert.Equal(t, originalPayload.ExpiredAt.Unix(), verifiedPayload.ExpiredAt.Unix())
	})

	t.Run("should fail with invalid token format", func(t *testing.T) {
		invalidTokens := []string{
			"",
			"invalid",
			"not.a.token",
			"v2.local.invalid",
			"completely-invalid-token-format",
		}

		for _, token := range invalidTokens {
			payload, err := maker.VerifyToken(token)

			assert.Error(t, err)
			assert.Nil(t, payload)
			assert.Equal(t, ErrInvalidToken, err)
		}
	})

	t.Run("should fail with expired token", func(t *testing.T) {
		userID := uuid.New()
		duration := -time.Hour // Expired 1 hour ago

		// Create expired token
		tokenString, _, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		// Try to verify expired token
		payload, err := maker.VerifyToken(tokenString)

		assert.Error(t, err)
		assert.Nil(t, payload)
		assert.Equal(t, ErrExpiredToken, err)
	})

	t.Run("should fail with token from different key", func(t *testing.T) {
		// Create token with first maker
		userID := uuid.New()
		duration := time.Hour
		tokenString, _, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		// Create second maker with different key
		differentKey := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
		differentMaker, err := NewPasetoMaker(differentKey)
		require.NoError(t, err)

		// Try to verify token with different maker
		payload, err := differentMaker.VerifyToken(tokenString)

		assert.Error(t, err)
		assert.Nil(t, payload)
		assert.Equal(t, ErrInvalidToken, err)
	})

}

func TestPasetoMaker_TokenLifecycle(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	maker, err := NewPasetoMaker(validKey)
	require.NoError(t, err)

	t.Run("complete token lifecycle", func(t *testing.T) {
		userID := uuid.New()
		duration := 2 * time.Second // Short duration for testing

		// 1. Create token
		tokenString, originalPayload, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		// 2. Immediately verify (should work)
		verifiedPayload, err := maker.VerifyToken(tokenString)
		require.NoError(t, err)
		assert.Equal(t, originalPayload.UserUUID, verifiedPayload.UserUUID)

		// 3. Wait for expiration
		time.Sleep(3 * time.Second)

		// 4. Try to verify expired token (should fail)
		expiredPayload, err := maker.VerifyToken(tokenString)
		assert.Error(t, err)
		assert.Nil(t, expiredPayload)
		assert.Equal(t, ErrExpiredToken, err)
	})
}

func TestPasetoMaker_Security(t *testing.T) {
	t.Run("different keys should produce incompatible tokens", func(t *testing.T) {
		key1 := "12345678901234567890123456789012"
		key2 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"

		maker1, err := NewPasetoMaker(key1)
		require.NoError(t, err)

		maker2, err := NewPasetoMaker(key2)
		require.NoError(t, err)

		userID := uuid.New()
		duration := time.Hour

		// Create token with maker1
		token1, _, err := maker1.CreateToken(userID, duration)
		require.NoError(t, err)

		// Create token with maker2
		token2, _, err := maker2.CreateToken(userID, duration)
		require.NoError(t, err)

		// Tokens should be different
		assert.NotEqual(t, token1, token2)

		// maker1 cannot verify maker2's token
		_, err = maker1.VerifyToken(token2)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidToken, err)

		// maker2 cannot verify maker1's token
		_, err = maker2.VerifyToken(token1)
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidToken, err)
	})

}

func TestPasetoMaker_EdgeCases(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	maker, err := NewPasetoMaker(validKey)
	require.NoError(t, err)

	t.Run("should handle nil user ID", func(t *testing.T) {
		// uuid.UUID zero value
		var nilUUID uuid.UUID
		duration := time.Hour

		tokenString, payload, err := maker.CreateToken(nilUUID, duration)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		assert.Equal(t, "00000000-0000-0000-0000-000000000000", payload.UserUUID)
	})

	t.Run("should handle very long duration", func(t *testing.T) {
		userID := uuid.New()
		duration := 100 * 365 * 24 * time.Hour // 100 years

		tokenString, payload, err := maker.CreateToken(userID, duration)

		require.NoError(t, err)
		assert.NotEmpty(t, tokenString)
		assert.True(t, payload.ExpiredAt.After(time.Now().Add(50*365*24*time.Hour)))
	})
}

func TestPasetoMaker_Performance(t *testing.T) {
	validKey := "12345678901234567890123456789012"
	maker, err := NewPasetoMaker(validKey)
	require.NoError(t, err)

	t.Run("should handle high token creation volume", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Hour

		start := time.Now()

		// Create 1000 tokens
		for i := 0; i < 1000; i++ {
			_, _, err := maker.CreateToken(userID, duration)
			require.NoError(t, err)
		}

		elapsed := time.Since(start)
		t.Logf("Created 1000 tokens in %v", elapsed)

		// Should be reasonably fast (less than 1 second for 1000 tokens)
		assert.Less(t, elapsed, time.Second, "Token creation should be fast")
	})

	t.Run("should handle high token verification volume", func(t *testing.T) {
		userID := uuid.New()
		duration := time.Hour

		// Create a token
		tokenString, _, err := maker.CreateToken(userID, duration)
		require.NoError(t, err)

		start := time.Now()

		// Verify the same token 1000 times
		for i := 0; i < 1000; i++ {
			_, err := maker.VerifyToken(tokenString)
			require.NoError(t, err)
		}

		elapsed := time.Since(start)
		t.Logf("Verified token 1000 times in %v", elapsed)

		// Should be reasonably fast
		assert.Less(t, elapsed, time.Second, "Token verification should be fast")
	})
}
