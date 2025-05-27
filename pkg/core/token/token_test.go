package token

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	t.Run("SaveToken with empty keyID", func(t *testing.T) {
		token, err := GenerateToken("", 0)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.NotEmpty(t, token.ID, "Token ID should not be empty")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
		fmt.Println(token.Secret)
		fmt.Println(len(getTokenPair(token.ID, token.Secret)))
		fmt.Println(len(token.ID))
		fmt.Println(len(token.Secret))
		assert.True(t, len(getTokenPair(token.ID, token.Secret))%3 == 0, "The string should be divisible by 3 for base64 encoding")
		assert.Equal(t, 3600*time.Second, token.TTL, "Token TTL should not be the default value")
	})

	t.Run("SaveToken with provided keyID and TTL", func(t *testing.T) {
		keyID := "testkeyid"
		token, err := GenerateToken(keyID, 100)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, keyID, token.ID, "Token ID should match the provided keyID")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
		assert.True(t, len(getTokenPair(token.ID, token.Secret))%3 == 0, "The string should be divisible by 3 for base64 encoding")
		assert.Equal(t, 100*time.Second, token.TTL, "Token TTL should not be the default value")
	})

	t.Run("unusually long keyID returns error", func(t *testing.T) {
		keyID := "testKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyID"
		token, err := GenerateToken(keyID, 0)
		assert.Error(t, err, "Token generation should return an error for unusually long keyID")
		assert.Nil(t, token, "Token should be nil on error")
	})

	t.Run("SaveToken with valid alphanumeric keyID", func(t *testing.T) {
		keyID := "abc123"
		token, err := GenerateToken(keyID, 0)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, keyID, token.ID, "Token ID should match the provided alphanumeric keyID")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
	})

	t.Run("SaveToken with unsupported characters", func(t *testing.T) {
		keyID := "INVALID_KEY!"
		token, err := GenerateToken(keyID, 0)
		assert.Error(t, err, "Token generation should return an error for unsupported characters in keyID")
		assert.Nil(t, token, "Token should be nil when keyID contains unsupported characters")
	})

	t.Run("SaveToken with negative TTL", func(t *testing.T) {
		keyID := "testkeyid"
		token, err := GenerateToken(keyID, -1)
		assert.Error(t, err, "Token generation should return an error for negative TTL")
		assert.Nil(t, token, "Token should be nil when TTL is negative")
	})
}

func TestEncode(t *testing.T) {
	t.Run("Encode token", func(t *testing.T) {
		token := &Token{
			ID:     "testID",
			Secret: "testSecret",
		}
		encoded := token.Encode()
		expected := base64.StdEncoding.EncodeToString([]byte("testID:testSecret"))
		assert.Equal(t, expected, encoded, "Encoded token should match the expected value")
	})
}

func TestDecode(t *testing.T) {
	t.Run("Decode valid token", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("testID:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "testID", token.ID, "Decoded token ID should match")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
	})

	t.Run("Decode invalid token format", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("invalidFormat"))
		_, err := Decode(encoded)
		assert.Error(t, err, "Decoding should return an error for invalid format")
		assert.Contains(t, err.Error(), "invalid token format", "Error message should indicate invalid format")
	})

	t.Run("Decode invalid base64 string", func(t *testing.T) {
		_, err := Decode("invalidBase64")
		assert.Error(t, err, "Decoding should return an error for invalid base64 string")
	})
}

func TestGenerateID(t *testing.T) {
	t.Run("GenerateID generates unique IDs", func(t *testing.T) {
		id1, err1 := generateID()
		id2, err2 := generateID()

		assert.NoError(t, err1, "ID generation should not return an error")
		assert.NoError(t, err2, "ID generation should not return an error")
		assert.NotEqual(t, id1, id2, "Generated IDs should be unique")
		assert.Len(t, id1, defaultIDLength, "Generated ID should have the correct length")
		assert.Len(t, id2, defaultIDLength, "Generated ID should have the correct length")
	})
}

func TestGenerateSecret(t *testing.T) {
	t.Run("GenerateSecret generates valid secret", func(t *testing.T) {
		buffer := 21
		secret, err := generateSecret(WebToken, buffer)
		assert.NoError(t, err, "Secret generation should not return an error")
		assert.Len(t, secret, buffer, "Generated Secret should have the correct length")
	})
}
