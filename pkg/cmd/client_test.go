package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunClientCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "Invalid log level",
			args: args{
				Token:    "dGVzdDp0ZXN0",
				Server:   "test-server",
				Expose:   "test-dest",
				NoTLS:    false,
				Insecure: false,
			},
			wantErr: "failed to init logger: slog: level string \"\": unknown name",
		},
		{
			name: "invalid token",
			args: args{
				Token:    "invalid-token",
				Server:   "test-server",
				Expose:   "test-dest",
				NoTLS:    false,
				Insecure: false,
				LogLevel: "info",
			},
			wantErr: "invalid token: illegal base64 data at input byte 7",
		},
		{
			name: "valid token",
			args: args{
				Token:    "dGVzdDp0ZXN0",
				Server:   "test-server",
				Expose:   "test-dest",
				NoTLS:    false,
				Insecure: false,
				LogLevel: "info",
			},
			wantErr: "failed to split host and port: address test-server: missing port in address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Act
			err := RunClientCommand(ctx, &tt.args)

			// Assert
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
