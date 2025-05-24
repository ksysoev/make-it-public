package auth

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRepo_Verify(t *testing.T) {
	tests := []struct {
		wantErr   error
		mockSetup func(m redismock.ClientMock)
		name      string
		keyID     string
		secret    string
		want      bool
	}{
		{
			name:   "valid key with matching secret",
			keyID:  "key123",
			secret: "secret123",
			mockSetup: func(m redismock.ClientMock) {
				val, err := hashSecret("secret123", []byte(""))
				assert.NoError(t, err)
				m.ExpectGet("prefixkey123").SetVal(val)
			},
			want:    true,
			wantErr: nil,
		},
		{
			name:   "valid key with non-matching secret",
			keyID:  "key123",
			secret: "invalidSecret",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectGet("prefixkey123").SetVal("secret123")
			},
			want:    false,
			wantErr: nil,
		},
		{
			name:   "key does not exist",
			keyID:  "key123",
			secret: "secret123",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectGet("prefixkey123").RedisNil()
			},
			want:    false,
			wantErr: nil,
		},
		{
			name:   "redis error",
			keyID:  "key123",
			secret: "secret123",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectGet("prefixkey123").SetErr(assert.AnError)
			},
			want:    false,
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb, mockRDB := redismock.NewClientMock()
			tt.mockSetup(mockRDB)

			r := &Repo{
				db:        rdb,
				keyPrefix: "prefix",
			}

			got, err := r.Verify(context.Background(), tt.keyID, tt.secret)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepo_SaveToken(t *testing.T) {
	matcher := func(_, _ []interface{}) error {
		return nil
	}

	tests := []struct {
		wantErr   error
		mockSetup func(m redismock.ClientMock)
		name      string
	}{
		{
			name: "successful token save",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(true)
			},
			wantErr: nil,
		},
		{
			name: "duplicate token ID",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(false)
			},
			wantErr: core.ErrDuplicateTokenID,
		},
		{
			name: "failed due to redis error",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetErr(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb, mockRDB := redismock.NewClientMock()
			tt.mockSetup(mockRDB)

			r := &Repo{
				db:        rdb,
				keyPrefix: "prefix",
				salt:      []byte("test-salt"),
			}

			// Create a test token
			testToken := &token.Token{
				ID:     "test-id",
				Secret: "test-secret",
			}

			err := r.SaveToken(context.Background(), testToken, time.Minute)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRepo_Close(t *testing.T) {
	rdb, _ := redismock.NewClientMock()
	r := &Repo{
		db: rdb,
	}

	err := r.Close()
	require.NoError(t, err)
}

func TestHashSecret(t *testing.T) {
	tests := []struct {
		wantErr   error
		name      string
		secret    string
		salt      []byte
		expectErr bool
	}{
		{
			name:      "valid secret and empty salt",
			secret:    "password123",
			salt:      []byte(""),
			wantErr:   nil,
			expectErr: false,
		},
		{
			name:      "valid secret and non-empty salt",
			secret:    "mypassword",
			salt:      []byte("somesalt"),
			wantErr:   nil,
			expectErr: false,
		},
		{
			name:      "empty secret",
			secret:    "",
			salt:      []byte("somesalt"),
			wantErr:   nil,
			expectErr: false,
		},
		{
			name:      "large salt value",
			secret:    "password123",
			salt:      []byte("verylargesaltvalueusedforhashing"),
			wantErr:   nil,
			expectErr: false,
		},
		{
			name:      "nil salt",
			secret:    "password123",
			salt:      nil,
			wantErr:   nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hashSecret(tt.secret, tt.salt)

			if tt.expectErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, got)
				assert.Contains(t, got, scryptPrefix)
			}
		})
	}
}

func TestRepo_DeleteToken(t *testing.T) {
	tests := []struct {
		wantErr   error
		mockSetup func(m redismock.ClientMock)
		name      string
		tokenID   string
	}{
		{
			name:    "successfully delete token",
			tokenID: "token123",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectDel("prefixtoken123").SetVal(1)
			},
			wantErr: nil,
		},
		{
			name:    "token does not exist",
			tokenID: "nonexistentToken",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectDel("prefixnonexistentToken").SetVal(0)
			},
			wantErr: core.ErrTokenNotFound,
		},
		{
			name:    "redis error during deletion",
			tokenID: "tokenWithError",
			mockSetup: func(m redismock.ClientMock) {
				m.ExpectDel("prefixtokenWithError").SetErr(assert.AnError)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			rdb, mockRDB := redismock.NewClientMock()
			tt.mockSetup(mockRDB)

			r := &Repo{
				db:        rdb,
				keyPrefix: "prefix",
			}

			// Act
			err := r.DeleteToken(context.Background(), tt.tokenID)

			// Assert
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		config    *Config
		name      string
		expectErr bool
	}{
		{
			name: "valid configuration",
			config: &Config{
				RedisAddr: "localhost:6379",
				Password:  "password",
				KeyPrefix: "test",
				Salt:      "test-salt",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.config)

			if tt.expectErr {
				assert.Nil(t, r)
			} else {
				assert.NotNil(t, r)
				assert.Equal(t, tt.config.KeyPrefix, r.keyPrefix)
				assert.Equal(t, []byte(tt.config.Salt), r.salt)
			}
		})
	}
}
