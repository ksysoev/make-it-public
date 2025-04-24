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
				m.ExpectGet("prefixkey123").SetVal("secret123")
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

			_, err := r.GenerateToken(context.Background(), time.Minute)
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
