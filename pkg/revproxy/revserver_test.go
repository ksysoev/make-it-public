package revproxy

import (
	"context"
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

	var expectedCert *Certificate

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
				assert.Equal(t, expectedCert, server.cert)
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

func TestRunWithModifiedTLS(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	cert, key, err := generateSelfSignedCert()
	require.NoError(t, err)

	err = os.WriteFile(certPath, cert, 0o600)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, key, 0o600)
	require.NoError(t, err)

	mockConnService := NewMockConnService(t)
	cfg := &Config{
		Listen: ":8081",
		Cert:   certPath,
		Key:    keyPath,
	}

	server, err := New(cfg, mockConnService)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConnService.EXPECT().
		HandleReverseConn(mock.Anything, mock.Anything).
		Return(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx)
	}()

	currentCert := server.cert

	time.Sleep(100 * time.Millisecond)

	// modify the certificate file to simulate a change
	cert, key, err = generateRSACert()
	assert.NoError(t, err)

	err = os.WriteFile(certPath, cert, 0o600)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, key, 0o600)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	newCert := server.cert

	assert.NotEqual(t, currentCert.Cert, newCert.Cert, "Certificate should be reloaded after modification")

	conn, err := net.Dial("tcp", server.listen)
	if err == nil {
		_ = conn.Close()
	}

	cancel()

	select {
	case err := <-errCh:
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

func generateRSACert() (cert, key []byte, err error) {
	// This is a placeholder. In a real implementation, you would generate
	// a self-signed certificate here. For the purpose of this test, we'll
	// return dummy values that will cause the test to fail in a controlled way.
	return []byte(`-----BEGIN CERTIFICATE-----
MIIDYDCCAkgCCQDLtp0ELa4EBDANBgkqhkiG9w0BAQsFADByMQswCQYDVQQGEwJt
eTERMA8GA1UECAwIc2VsYW5nb3IxEjAQBgNVBAcMCWN5YmVyamF5YTEXMBUGA1UE
CgwObWFrZS1pdC1wdWJsaWMxIzAhBgkqhkiG9w0BCQEWFG1oMThhYjM2MzJAZ21h
aWwuY29tMB4XDTI1MDYxMjA4MDExOVoXDTM1MDYxMDA4MDExOVowcjELMAkGA1UE
BhMCbXkxETAPBgNVBAgMCHNlbGFuZ29yMRIwEAYDVQQHDAljeWJlcmpheWExFzAV
BgNVBAoMDm1ha2UtaXQtcHVibGljMSMwIQYJKoZIhvcNAQkBFhRtaDE4YWIzNjMy
QGdtYWlsLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAJbQU0xy
smQyXizcoBPykl5lxn1/ZDYY3rgx+47KLp9HlubKy8PA8c1Nwg2T6CyrKA1+BFOt
87S89lpqJ5+7VNN4FfCeWWqLhwPGdIrw+1i93RAjATMdg2S/eHQ6yReSJ/LFQgfs
KKYxmi1S7l1yG7LKbthbOsFHj2JKK6UHSK/eup+X5EwDoV6GR9bgG0fhNSmLwLtV
tdAxScvypJvn4np2mQWPbpASleg88DdRqDGaAo+3Ih1+k/wrsXmuP0mMUZK7tt7j
/Sg5Upqlfvks68zVXvO1msLc2DsUq7bMGDa/iz/q/fIsE2x8Rx/8GiAeUiSzg9fM
KmHVIPopevhRx3ECAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAabxgrWJ5Z5tL3G/u
q1D3V/7z5xA2kQNZ85mKPexZ5lJw5aT3U9+1OOj8FQM6I7HtF5z9qE+Efi4DnLpd
EZhFVnhHso9UFqhfgQpE7CY+nLaALv3QNneSlO/qFDkyF5r5Dw/115BDdeq3+4Lh
dCU5WqUf1jSazXwzHaVa2YjGCnZghyUtSQCIPpzp16e61TK5wt8kC8pR/rhYkSxo
B1aHw0B/u/lJTWGIIA3LwGZw6d6k1ltoOCza5cowKdy4I9xo4vCMoRtp/MtjktXt
VHvXeZktKLFoOc+77uqj2LmxFV2+KZ65C77WhFHBYcF96mDx7kiRR4FyxZ7V+63z
xMyYhQ==
-----END CERTIFICATE-----`),
		[]byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCW0FNMcrJkMl4s
3KAT8pJeZcZ9f2Q2GN64MfuOyi6fR5bmysvDwPHNTcINk+gsqygNfgRTrfO0vPZa
aiefu1TTeBXwnllqi4cDxnSK8PtYvd0QIwEzHYNkv3h0OskXkifyxUIH7CimMZot
Uu5dchuyym7YWzrBR49iSiulB0iv3rqfl+RMA6FehkfW4BtH4TUpi8C7VbXQMUnL
8qSb5+J6dpkFj26QEpXoPPA3UagxmgKPtyIdfpP8K7F5rj9JjFGSu7be4/0oOVKa
pX75LOvM1V7ztZrC3Ng7FKu2zBg2v4s/6v3yLBNsfEcf/BogHlIks4PXzCph1SD6
KXr4UcdxAgMBAAECggEAR2rx92LdXZuIg2AbIjcd3zv9ChMYppGSbtGkmdLezyi8
qiBg7BtjpmBrQ7jGGtkWh4UkkWfv36gYVWqtxvOUUOwuH5stJspaLox9RgqmTDjl
Ba499DHGtiAB77Ci59mbt1h4U34fJcyZgVsja/cMbNd2NFjHcx3rJZWQI39WiYmK
56rzVMpCJRlvre1Duvl/hFaDhAYYlyGsfVRb+R9+INkvP/obxTLrScNE6HxFthgO
OvxL2vp6G87LndnwTfUWC0+h7hn7n3OXsgECmWKETgQL7bfcQITvYM0NmRWl/Ol4
I0YXMOcpdkZ3ry4LeukvuZE/bjhHWaCspeDAyTDuvQKBgQDE2fGnkYOdu4Ll5P96
lXfow7/9qIofUmp5jOquKwEhCtHUbSmOeM7pw+h3PGwYV2hXEYHZdTOJ/aU3WI4S
656UN1ACA5iPA84Dr3RklptaE/bBTHosQ+FfFMHnbsuj0t2j02k2ZYNzAaXG01TD
1hQch8ddrUaMVQWoV8CckGyhkwKBgQDEIRzasboNjpv4RnxAYe0mkfnbc8mLm/1X
QOWFiVBYhpKUIe9G4mA73M8G6Py5CQplGQqGCtaVG2nIzxQ6UxyAeETvcx8z5O0/
6MkdRNKa3Zkvwc+2IANNWduijQzPiMRfgMfOu/IlDbwdHo26PV1Cx68oZ+lbSRE+
l3XSdDAlawKBgQCoZQR7Y1ijEygr/9SpCbn07ZeMp6PYnYkmB+0uJu2lVXsgbG2z
Shc/FG8FqTOTMxq3+OsKml8HeWrfSKro9pTGl/aicm8MUKXosywvbELjMNbSjtio
izz9OGWT1EzyDM27enuzo+1p8Yvd5STLDpRPv7tFoJgMLiNT2hWUGVxEbwKBgESi
7e0e22SZLr4hNKR3YL3pwg3ppHPGIE+jt28XEdYZKjzK72jYGiN4776UVLUQk+Gz
dLpaGqRN1qRey85pfYT8EevWVuobSGfgOFmU1zs5J73NzroG1AEC3FkzkXMjgs2F
TOdtYJ1VBCsQoTq29OdE6Gh0jPbUSEOmT6ZZ4OuHAoGAWyU4JIxGfP1mQlA6QYu8
5FFd1ldbaid+DnHoiv1gdXuM2RGXXyf5LEXnVwlodQJ4ANKmpmto5QdhKmRHBhD0
uRdt2i8umLKY+bF4lgCaLBbe3gzoUzxp3rvOP3hNNMltZAkBjRSw05MWyMDBPiKh
XIU2boodjWnKYPEmWzgtJw4=
-----END PRIVATE KEY-----`),
		nil
}
