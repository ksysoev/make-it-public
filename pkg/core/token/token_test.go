package token

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	t.Run("SaveToken with empty keyID", func(t *testing.T) {
		token, err := GenerateToken("", 0, TokenTypeWeb)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.NotEmpty(t, token.ID, "Token ID should not be empty")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
		assert.Equal(t, TokenTypeWeb, token.Type, "Token type should be Web")
		// Note: The encoded format now includes type prefix, so we skip the divisibility check
		assert.Equal(t, 3600*time.Second, token.TTL, "Token TTL should not be the default value")
	})

	t.Run("SaveToken with provided keyID and TTL", func(t *testing.T) {
		keyID := "testkeyid"
		token, err := GenerateToken(keyID, 100, TokenTypeTCP)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, keyID, token.ID, "Token ID should match the provided keyID")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
		assert.Equal(t, TokenTypeTCP, token.Type, "Token type should be TCP")
		assert.Equal(t, 100*time.Second, token.TTL, "Token TTL should not be the default value")
	})

	t.Run("SaveToken defaults to web type when empty", func(t *testing.T) {
		keyID := "testkey"
		token, err := GenerateToken(keyID, 100, "")
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, TokenTypeWeb, token.Type, "Token type should default to Web")
	})

	t.Run("SaveToken rejects invalid token type", func(t *testing.T) {
		keyID := "testkey"
		token, err := GenerateToken(keyID, 100, "invalid")
		assert.Error(t, err, "Token generation should return an error for invalid type")
		assert.Nil(t, token, "Token should be nil on error")
		assert.ErrorIs(t, err, ErrInvalidTokenType)
	})

	t.Run("unusually long keyID returns error", func(t *testing.T) {
		keyID := "testKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyIDtestKeyID"
		token, err := GenerateToken(keyID, 0, TokenTypeWeb)
		assert.Error(t, err, "Token generation should return an error for unusually long keyID")
		assert.Nil(t, token, "Token should be nil on error")
	})

	t.Run("SaveToken with valid alphanumeric keyID", func(t *testing.T) {
		keyID := "abc123"
		token, err := GenerateToken(keyID, 0, TokenTypeWeb)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, keyID, token.ID, "Token ID should match the provided alphanumeric keyID")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
	})

	t.Run("SaveToken with unsupported characters", func(t *testing.T) {
		keyID := "INVALID_KEY!"
		token, err := GenerateToken(keyID, 0, TokenTypeWeb)
		assert.Error(t, err, "Token generation should return an error for unsupported characters in keyID")
		assert.Nil(t, token, "Token should be nil when keyID contains unsupported characters")
	})

	t.Run("SaveToken with negative TTL", func(t *testing.T) {
		keyID := "testkeyid"
		token, err := GenerateToken(keyID, -1, TokenTypeWeb)
		assert.Error(t, err, "Token generation should return an error for negative TTL")
		assert.Nil(t, token, "Token should be nil when TTL is negative")
	})
}

func TestEncode(t *testing.T) {
	t.Run("Encode web token", func(t *testing.T) {
		token := &Token{
			ID:     "testID",
			Secret: "testSecret",
			Type:   TokenTypeWeb,
		}
		encoded := token.Encode()
		expected := base64.StdEncoding.EncodeToString([]byte("w:testID:testSecret"))
		assert.Equal(t, expected, encoded, "Encoded token should match the expected value with type prefix")
	})

	t.Run("Encode TCP token", func(t *testing.T) {
		token := &Token{
			ID:     "testID",
			Secret: "testSecret",
			Type:   TokenTypeTCP,
		}
		encoded := token.Encode()
		expected := base64.StdEncoding.EncodeToString([]byte("t:testID:testSecret"))
		assert.Equal(t, expected, encoded, "Encoded token should match the expected value with type prefix")
	})

	t.Run("Encode token without type defaults to web", func(t *testing.T) {
		token := &Token{
			ID:     "testID",
			Secret: "testSecret",
		}
		encoded := token.Encode()
		expected := base64.StdEncoding.EncodeToString([]byte("w:testID:testSecret"))
		assert.Equal(t, expected, encoded, "Encoded token should default to web type")
	})
}

func TestDecode(t *testing.T) {
	t.Run("Decode valid web token", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("w:testID:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "testID", token.ID, "Decoded token ID should match")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
		assert.Equal(t, TokenTypeWeb, token.Type, "Decoded token type should be Web")
	})

	t.Run("Decode valid TCP token", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("t:testID:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "testID", token.ID, "Decoded token ID should match")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
		assert.Equal(t, TokenTypeTCP, token.Type, "Decoded token type should be TCP")
	})

	t.Run("Decode old token without type prefix defaults to web", func(t *testing.T) {
		// Old format without type prefix - 2-part format
		encoded := base64.StdEncoding.EncodeToString([]byte("abc123:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "abc123", token.ID, "Decoded token ID should match")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
		assert.Equal(t, TokenTypeWeb, token.Type, "Decoded token type should default to Web for backward compatibility")
	})

	t.Run("Decode old token with ID starting with 'w' defaults to web", func(t *testing.T) {
		// Old format where ID itself starts with 'w' - should not be treated as a type prefix
		encoded := base64.StdEncoding.EncodeToString([]byte("wabc123:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "wabc123", token.ID, "Decoded token ID should preserve leading 'w'")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
		assert.Equal(t, TokenTypeWeb, token.Type, "Decoded token type should default to Web for old-format tokens")
	})

	t.Run("Decode old token with ID starting with 't' defaults to web", func(t *testing.T) {
		// Old format where ID itself starts with 't' - should not be treated as a type prefix
		encoded := base64.StdEncoding.EncodeToString([]byte("tabc123:testSecret"))
		token, err := Decode(encoded)
		assert.NoError(t, err, "Decoding should not return an error")
		assert.Equal(t, "tabc123", token.ID, "Decoded token ID should preserve leading 't'")
		assert.Equal(t, "testSecret", token.Secret, "Decoded token Secret should match")
		assert.Equal(t, TokenTypeWeb, token.Type, "Decoded token type should default to Web for old-format tokens")
	})

	t.Run("Decode old token with common IDs starting with 'w' or 't'", func(t *testing.T) {
		// Test realistic user-provided keyIDs that start with 'w' or 't'
		testCases := []struct {
			name   string
			keyID  string
			secret string
		}{
			{"webserver", "webserver", "secretABC"},
			{"testkey", "testkey", "secretDEF"},
			{"tunnel", "tunnel", "secretGHI"},
			{"worker", "worker", "secretJKL"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Old format: <ID>:<Secret>
				encoded := base64.StdEncoding.EncodeToString([]byte(tc.keyID + ":" + tc.secret))
				token, err := Decode(encoded)
				assert.NoError(t, err, "Decoding should not return an error")
				assert.Equal(t, tc.keyID, token.ID, "Decoded token ID should match exactly")
				assert.Equal(t, tc.secret, token.Secret, "Decoded token Secret should match")
				assert.Equal(t, TokenTypeWeb, token.Type, "Decoded token type should default to Web")
			})
		}
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

	t.Run("Decode invalid token type in new format", func(t *testing.T) {
		// New format with invalid type
		encoded := base64.StdEncoding.EncodeToString([]byte("x:testID:testSecret"))
		_, err := Decode(encoded)
		assert.Error(t, err, "Decoding should return an error for invalid token type")
		assert.Contains(t, err.Error(), "invalid token type", "Error message should indicate invalid type")
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
		secret, err := generateSecret(buffer)
		assert.NoError(t, err, "Secret generation should not return an error")
		assert.Len(t, secret, buffer, "Generated Secret should have the correct length")
	})
}

func TestTokenTypeString(t *testing.T) {
	t.Run("TokenTypeWeb String() returns 'web'", func(t *testing.T) {
		assert.Equal(t, "web", TokenTypeWeb.String(), "TokenTypeWeb.String() should return 'web'")
	})

	t.Run("TokenTypeTCP String() returns 'tcp'", func(t *testing.T) {
		assert.Equal(t, "tcp", TokenTypeTCP.String(), "TokenTypeTCP.String() should return 'tcp'")
	})

	t.Run("Invalid TokenType String() returns raw value", func(t *testing.T) {
		invalid := TokenType("x")
		assert.Equal(t, "x", invalid.String(), "Invalid TokenType.String() should return raw value")
	})
}
