package token

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	t.Run("GenerateToken with empty keyID", func(t *testing.T) {
		token, err := GenerateToken("")
		assert.NoError(t, err, "Token generation should not return an error")
		assert.NotEmpty(t, token.ID, "Token ID should not be empty")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
	})

	t.Run("GenerateToken with provided keyID", func(t *testing.T) {
		keyID := "testKeyID"
		token, err := GenerateToken(keyID)
		assert.NoError(t, err, "Token generation should not return an error")
		assert.Equal(t, keyID, token.ID, "Token ID should match the provided keyID")
		assert.NotEmpty(t, token.Secret, "Token Secret should not be empty")
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
		assert.Len(t, id1, idLength, "Generated ID should have the correct length")
		assert.Len(t, id2, idLength, "Generated ID should have the correct length")
	})
}

func TestGenerateSecret(t *testing.T) {
	t.Run("GenerateSecret generates valid secret", func(t *testing.T) {
		secret, err := generateSecret()
		assert.NoError(t, err, "Secret generation should not return an error")
		assert.Len(t, secret, secretLength, "Generated Secret should have the correct length")
	})
}
