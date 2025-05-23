package auth

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
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

func TestRepo_GenerateToken(t *testing.T) {
	matcher := func(_, _ []interface{}) error {
		return nil
	}

	tests := []struct {
		wantErr   error
		mockSetup func(m redismock.ClientMock)
		name      string
	}{
		{
			name: "successful token generation",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(true)
			},
			wantErr: nil,
		},
		{
			name: "token collision resolved after retry",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(false)
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(true)
			},
			wantErr: nil,
		},
		{
			name: "all attempts failed due to token collision",
			mockSetup: func(m redismock.ClientMock) {
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(false)
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(false)
				m.CustomMatch(matcher).ExpectSetNX(mock.Anything, mock.Anything, time.Minute).SetVal(false)
			},
			wantErr: ErrFailedToGenerateToken,
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
			}

			_, err := r.GenerateToken(context.Background(), "", time.Minute)
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
		name      string
		tokenID   string
		mockSetup func(m redismock.ClientMock)
		wantErr   error
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
			wantErr: nil,
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
