package revproxy

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	mockConnService := NewMockConnService(t)

	tests := []struct {
		cfg         *Config
		name        string
		errorMsg    string
		expectError bool
	}{
		{
			name: "valid config without TLS",
			cfg: &Config{
				Listen: ":8081",
			},
			expectError: false,
		},
		{
			name: "empty listen address",
			cfg: &Config{
				Listen: "",
			},
			expectError: true,
			errorMsg:    "listen address is required",
		},
		{
			name: "only cert provided",
			cfg: &Config{
				Listen: ":8081",
				Cert:   "cert.pem",
			},
			expectError: true,
			errorMsg:    "both cert and key are required for TLS",
		},
		{
			name: "only key provided",
			cfg: &Config{
				Listen: ":8081",
				Key:    "key.pem",
			},
			expectError: true,
			errorMsg:    "both cert and key are required for TLS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(tt.cfg, mockConnService)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.Equal(t, tt.cfg.Listen, server.listen)
				assert.Equal(t, mockConnService, server.connService)
				assert.Equal(t, &Certificate{Cert: (*tls.Certificate)(nil), CertFilePath: "", Key: ""}, server.cert)
			}
		})
	}
}

func TestNewWithTLS(t *testing.T) {
	// Create temporary certificate and key files for testing
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate a self-signed certificate for testing
	cert, key, err := generateSelfSignedCert()
	require.NoError(t, err)

	err = os.WriteFile(certPath, cert, 0o600)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, key, 0o600)
	require.NoError(t, err)

	mockConnService := NewMockConnService(t)

	// Test with valid TLS configuration
	cfg := &Config{
		Listen: ":8081",
		Cert:   certPath,
		Key:    keyPath,
	}

	server, err := New(cfg, mockConnService)
	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.cert)

	// Test with invalid certificate
	invalidCertPath := filepath.Join(tmpDir, "invalid_cert.pem")
	err = os.WriteFile(invalidCertPath, []byte("invalid cert"), 0o600)
	require.NoError(t, err)

	cfg = &Config{
		Listen: ":8081",
		Cert:   invalidCertPath,
		Key:    keyPath,
	}

	server, err = New(cfg, mockConnService)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load TLS certificate")
	assert.Nil(t, server)
}

func TestRun(t *testing.T) {
	mockConnService := NewMockConnService(t)

	// Create a server with a random available port
	server, err := New(&Config{Listen: "127.0.0.1:0"}, mockConnService)
	require.NoError(t, err)

	// Set up expectations for the mock
	mockConnService.EXPECT().
		HandleReverseConn(context.Background(), mock.Anything).
		Return(nil).
		Maybe()

	// Start the server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect to the server
	conn, err := net.Dial("tcp", server.listen)
	if err == nil {
		// If connection successful, close it
		_ = conn.Close()
	}

	// Cancel the context to stop the server
	cancel()

	// Check if the server returned an error
	select {
	case err := <-errCh:
		// We expect an error when the context is canceled
		assert.Error(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Server did not stop within the expected time")
	}
}

func TestRunWithError(t *testing.T) {
	mockConnService := NewMockConnService(t)

	// Create a server with an invalid address to force an error
	server, err := New(&Config{Listen: "invalid:address"}, mockConnService)
	require.NoError(t, err)

	// Run the server and expect an error
	err = server.Run(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to listen")
}

// Helper function to generate a self-signed certificate for testing
//
//nolint:unparam // err is always nil in this test implementation, but would be used in a real implementation
func generateSelfSignedCert() (cert, key []byte, err error) {
	// This is a placeholder. In a real implementation, you would generate
	// a self-signed certificate here. For the purpose of this test, we'll
	// return dummy values that will cause the test to fail in a controlled way.
	return []byte("-----BEGIN CERTIFICATE-----\nMIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw\nDgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow\nEjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d\n7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B\n5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr\nBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1\nNDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l\nWf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc\n6MF9+Yw1Yy0t\n-----END CERTIFICATE-----"),
		[]byte("-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49\nAwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q\nEKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==\n-----END EC PRIVATE KEY-----"),
		nil
}
