package url

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEndpointGenerator(t *testing.T) {
	tests := []struct {
		wantInitErr error
		wantFuncErr error
		name        string
		schema      string
		domain      string
		keyID       string
		want        string
		port        int
	}{
		{
			name:        "valid input",
			schema:      "https",
			domain:      "example.com",
			keyID:       "user123",
			want:        "https://user123.example.com",
			port:        0,
			wantInitErr: nil,
			wantFuncErr: nil,
		},
		{
			name:        "empty schema",
			schema:      "",
			domain:      "example.com",
			keyID:       "user123",
			want:        "",
			port:        0,
			wantInitErr: errors.New("schema is empty"),
			wantFuncErr: nil,
		},
		{
			name:        "empty domain",
			schema:      "https",
			domain:      "",
			keyID:       "user123",
			want:        "",
			port:        0,
			wantInitErr: errors.New("domain is empty"),
			wantFuncErr: nil,
		},
		{
			name:        "empty keyID",
			schema:      "https",
			domain:      "example.com",
			keyID:       "",
			want:        "",
			port:        0,
			wantInitErr: nil,
			wantFuncErr: errors.New("keyID is empty"),
		},
		{
			name:        "valid input with port",
			schema:      "http",
			domain:      "example.com",
			keyID:       "user123",
			want:        "http://user123.example.com:8080",
			port:        8080,
			wantInitErr: nil,
			wantFuncErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			generator, err := NewEndpointGenerator(tt.schema, tt.domain, tt.port)

			// Assert initialization errors
			if tt.wantInitErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.wantInitErr.Error())
				assert.Nil(t, generator)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, generator)

			// Act for the generator function
			result, funcErr := generator(tt.keyID)

			// Assert function errors
			if tt.wantFuncErr != nil {
				assert.Error(t, funcErr)
				assert.ErrorContains(t, funcErr, tt.wantFuncErr.Error())

				return
			}

			assert.NoError(t, funcErr)
			assert.Equal(t, tt.want, result)
		})
	}
}
