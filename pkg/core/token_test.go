package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_GenerateToken(t *testing.T) {
	t.Run("successful token generation with empty keyID", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)

		// Mock expectations
		mockAuth.EXPECT().SaveToken(context.Background(),
			mock.MatchedBy(func(t *token.Token) bool {
				return t.ID != "" && t.Secret != "" && t.TTL == 3600*time.Second
			})).Return(nil)

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), "", 0)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, tkn.ID)
		assert.NotEmpty(t, tkn.Secret)
		assert.Equal(t, 3600*time.Second, tkn.TTL)
	})

	t.Run("successful token generation with provided keyID and TTL", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		keyID := "testkeyid"
		ttl := 100

		// Mock expectations
		mockAuth.EXPECT().SaveToken(context.Background(),
			mock.MatchedBy(func(t *token.Token) bool {
				return t.ID == keyID && t.Secret != "" && t.TTL == 100*time.Second
			})).Return(nil)

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), keyID, ttl)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, keyID, tkn.ID)
		assert.NotEmpty(t, tkn.Secret)
		assert.Equal(t, 100*time.Second, tkn.TTL)
	})

	t.Run("error from token generation - invalid characters", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		keyID := "INVALID_KEY!" // Contains invalid characters

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), keyID, 0)

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to generate token")
		assert.ErrorIs(t, err, token.ErrTokenInvalid)
	})

	t.Run("error from token generation - token too long", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		keyID := "thisistoolongforatokenid" // Exceeds maxIDLength

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), keyID, 0)

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to generate token")
		assert.ErrorIs(t, err, token.ErrTokenTooLong)
	})

	t.Run("error from token generation - invalid TTL", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), "validkeyid", -1) // Negative TTL

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to generate token")
		assert.ErrorIs(t, err, token.ErrInvalidTokenTTL)
	})

	t.Run("error from SaveToken", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		expectedErr := errors.New("database error")

		// Mock expectations
		mockAuth.EXPECT().SaveToken(context.Background(),
			mock.MatchedBy(func(t *token.Token) bool {
				return t.ID != "" && t.Secret != ""
			})).Return(expectedErr)

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), "", 0)

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to save token")
	})

	t.Run("duplicate token ID with non-empty keyID", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		keyID := "testkeyid"

		// Mock expectations
		mockAuth.EXPECT().SaveToken(context.Background(),
			mock.MatchedBy(func(t *token.Token) bool {
				return t.ID == keyID
			})).Return(ErrDuplicateTokenID)

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), keyID, 0)

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to save token")
		assert.ErrorIs(t, err, ErrDuplicateTokenID)
	})

	t.Run("duplicate token ID with empty keyID should retry", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)

		// Use a counter to simulate different behavior on different calls
		callCount := 0

		mockAuth.EXPECT().SaveToken(context.Background(),
			mock.MatchedBy(func(t *token.Token) bool {
				return t.ID != "" && t.Secret != ""
			})).RunAndReturn(func(_ context.Context, _ *token.Token) error {
			callCount++
			if callCount == 1 {
				return ErrDuplicateTokenID
			}

			return nil
		})

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), "", 0)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, tkn.ID)
		assert.NotEmpty(t, tkn.Secret)
		assert.Equal(t, 2, callCount, "SaveToken should be called exactly twice")
	})

	t.Run("retry exhaustion", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)

		// All attempts return duplicate token ID
		for i := 0; i < attemptsToGenerateToken; i++ {
			mockAuth.EXPECT().SaveToken(context.Background(),
				mock.MatchedBy(func(t *token.Token) bool {
					return t.ID != "" && t.Secret != ""
				})).Return(ErrDuplicateTokenID)
		}

		// Execute
		tkn, err := svc.GenerateToken(context.Background(), "", 0)

		// Assert
		require.Error(t, err)
		assert.Nil(t, tkn)
		assert.Contains(t, err.Error(), "failed to generate token after")
	})
}

func TestService_DeleteToken(t *testing.T) {
	t.Run("successful token deletion", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		tokenID := "test-token-id"

		// Mock expectations
		mockAuth.EXPECT().DeleteToken(context.Background(), tokenID).Return(nil)

		// Execute
		err := svc.DeleteToken(context.Background(), tokenID)

		// Assert
		require.NoError(t, err)
	})

	t.Run("error from DeleteToken", func(t *testing.T) {
		// Setup
		mockAuth := NewMockAuthRepo(t)
		svc := New(nil, mockAuth)
		tokenID := "test-token-id"
		expectedErr := ErrTokenNotFound

		// Mock expectations
		mockAuth.EXPECT().DeleteToken(context.Background(), tokenID).Return(expectedErr)

		// Execute
		err := svc.DeleteToken(context.Background(), tokenID)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})
}
