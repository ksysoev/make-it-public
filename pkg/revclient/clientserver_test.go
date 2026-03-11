package revclient

import (
	"context"
	"testing"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildOptsOptionCount enumerates the expected number of ListenerOption values
// returned by buildOpts for each configuration.
//
// Base options always present (regardless of config):
//
//  1. WithUserPass       — authentication
//  2. WithEventHandler   — urlToConnectUpdated event
//  3. WithListenerKeepAlive — TCP keepalive (the fix for #283)
//
// Conditional options:
//
//	+1 WithListenerTLSConfig  when NoTLS == false (default)
//	+1 WithEnableV2           when EnableV2 == true
const baseOptionCount = 3 // auth + event handler + keepalive

func newTestToken(t *testing.T) *token.Token {
	t.Helper()

	tkn, err := token.GenerateToken("testkey", 3600, token.TokenTypeWeb)
	require.NoError(t, err)

	return tkn
}

func TestBuildOpts_OptionCount(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantCount   int
		isReconnect bool
	}{
		{
			name:      "no TLS, no V2",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: false},
			wantCount: baseOptionCount,
		},
		{
			name:      "TLS enabled, no V2",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: false, EnableV2: false},
			wantCount: baseOptionCount + 1, // +WithListenerTLSConfig
		},
		{
			name:      "no TLS, V2 enabled",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: true},
			wantCount: baseOptionCount + 1, // +WithEnableV2
		},
		{
			name:      "TLS enabled, V2 enabled",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: false, EnableV2: true},
			wantCount: baseOptionCount + 2, // +WithListenerTLSConfig +WithEnableV2
		},
		{
			name:        "reconnect flag does not change option count",
			cfg:         Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: false},
			wantCount:   baseOptionCount,
			isReconnect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewClientServer(tt.cfg, newTestToken(t))
			opts, err := cs.buildOpts(context.Background(), tt.isReconnect)

			require.NoError(t, err)
			assert.Len(t, opts, tt.wantCount,
				"expected %d ListenerOption(s) including WithListenerKeepAlive", tt.wantCount)
		})
	}
}

func TestBuildOpts_InvalidServerAddr(t *testing.T) {
	// When TLS is enabled but the server address has no port, SplitHostPort should fail.
	cs := NewClientServer(Config{ServerAddr: "no-port-here", NoTLS: false}, newTestToken(t))

	_, err := cs.buildOpts(context.Background(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to split host and port")
}
