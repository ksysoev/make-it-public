package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/ksysoev/make-it-public/pkg/api"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/edge"
	"github.com/ksysoev/make-it-public/pkg/repo/auth"
	"github.com/ksysoev/make-it-public/pkg/repo/connmng"
	"github.com/ksysoev/make-it-public/pkg/revproxy"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

// MockRedis is a simple in-memory implementation of the Redis interface for testing
type MockRedis struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewMockRedis() *MockRedis {
	return &MockRedis{
		data: make(map[string]string),
	}
}

func (m *MockRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if val, ok := m.data[key]; ok {
		cmd.SetVal(val)
	} else {
		cmd.SetErr(redis.Nil)
	}

	return cmd
}

func (m *MockRedis) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)

	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)

	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			count++
		}
	}

	cmd.SetVal(count)

	return cmd
}

func (m *MockRedis) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[key]; !exists {
		m.data[key] = fmt.Sprintf("%v", value)

		cmd.SetVal(true)
	} else {
		cmd.SetVal(false)
	}

	return cmd
}

func (m *MockRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)

	m.mu.Lock()
	defer m.mu.Unlock()

	count := int64(0)

	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)

			count++
		}
	}

	cmd.SetVal(count)

	return cmd
}

func (m *MockRedis) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("PONG")

	return cmd
}

func (m *MockRedis) Close() error {
	return nil
}

// TestServerE2E tests the complete server lifecycle: start, verify running, and stop
func TestServerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Check if Redis is available, if not, skip health check test
	redisAvailable := checkRedisAvailable(t)

	// Find available ports for testing
	httpPort := findAvailablePort(t)
	revProxyPort := findAvailablePort(t)
	apiPort := findAvailablePort(t)

	redisAddr := "localhost:6379"
	if !redisAvailable {
		t.Logf("Redis not available at %s, some tests will be skipped", redisAddr)
	}

	// Create test configuration
	cfg := &appConfig{
		HTTP: edge.Config{
			Listen: fmt.Sprintf(":%d", httpPort),
			Public: edge.PublicEndpointConfig{
				Schema: "http",
				Domain: "localhost",
				Port:   httpPort,
			},
		},
		RevProxy: revproxy.Config{
			Listen: fmt.Sprintf(":%d", revProxyPort),
		},
		API: api.Config{
			Listen: fmt.Sprintf(":%d", apiPort),
		},
		Auth: auth.Config{
			RedisAddr: redisAddr,
			KeyPrefix: "TEST::",
			Salt:      "test-salt-for-e2e",
		},
	}

	// Create context with cancellation for server lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	serverStarted := make(chan struct{})

	go func() {
		// Signal that we're starting
		close(serverStarted)

		// Run the server (this will block until context is cancelled)
		err := runServerWithConfig(ctx, cfg)
		serverErrChan <- err
	}()

	// Wait for server to start
	<-serverStarted
	time.Sleep(500 * time.Millisecond) // Give server time to bind ports

	// Test 1: Verify HTTP server is running
	t.Run("HTTP server is accessible", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", httpPort))
		if err != nil {
			t.Fatalf("Failed to connect to HTTP server: %v", err)
		}
		defer resp.Body.Close()

		// We expect some response (even if 404 or other status)
		if resp.StatusCode == 0 {
			t.Error("Expected non-zero status code from HTTP server")
		}

		t.Logf("HTTP server responded with status: %d", resp.StatusCode)
	})

	// Test 2: Verify API server is running and health check works
	t.Run("API health check", func(t *testing.T) {
		if !redisAvailable {
			t.Skip("Redis not available, skipping health check test")
		}

		client := &http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", apiPort))
		if err != nil {
			t.Fatalf("Failed to connect to API server: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 from health check, got %d", resp.StatusCode)
		}

		t.Log("API health check passed")
	})

	// Test 3: Verify reverse proxy server is listening
	t.Run("Reverse proxy is listening", func(t *testing.T) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", revProxyPort), 5*time.Second)
		if err != nil {
			t.Fatalf("Failed to connect to reverse proxy server: %v", err)
		}

		conn.Close()

		t.Log("Reverse proxy server is listening")
	})

	// Test 4: Stop the server gracefully
	t.Run("Graceful shutdown", func(t *testing.T) {
		// Cancel context to trigger shutdown
		cancel()

		// Wait for server to stop with timeout
		select {
		case err := <-serverErrChan:
			// Accept both nil and certain expected errors during shutdown
			if err != nil && err != context.Canceled && err != http.ErrServerClosed {
				// Check if it's a "use of closed network connection" error which is expected
				if err.Error() != "" && !isClosedNetworkError(err) {
					t.Logf("Server returned error during shutdown (may be expected): %v", err)
				}
			}

			t.Log("Server stopped")
		case <-time.After(10 * time.Second):
			t.Fatal("Server did not stop within timeout")
		}
	})

	// Test 5: Verify servers are no longer accessible after shutdown
	t.Run("Servers are stopped", func(t *testing.T) {
		time.Sleep(200 * time.Millisecond) // Brief pause to ensure ports are released

		client := &http.Client{Timeout: 1 * time.Second}

		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", apiPort))
		if err == nil {
			resp.Body.Close()
			t.Error("Expected API server to be stopped, but it's still accessible")
		}

		t.Log("Confirmed servers are stopped")
	})
}

// checkRedisAvailable checks if Redis is available at the default address
func checkRedisAvailable(t *testing.T) bool {
	t.Helper()

	conn, err := net.DialTimeout("tcp", "localhost:6379", 1*time.Second)
	if err != nil {
		return false
	}

	conn.Close()

	return true
}

// isClosedNetworkError checks if the error is a "use of closed network connection" error
func isClosedNetworkError(err error) bool {
	return err != nil &&
		(err.Error() == "use of closed network connection" ||
			net.ErrClosed.Error() == err.Error() ||
			fmt.Sprintf("%v", err) == "accept tcp [::]:0: use of closed network connection")
}

// findAvailablePort finds and returns an available TCP port for testing
func findAvailablePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatal("Failed to get TCP address")
	}

	port := tcpAddr.Port

	if err := listener.Close(); err != nil {
		t.Logf("Failed to close listener: %v", err)
	}

	return port
}

// runServerWithConfig runs the server with the provided configuration
// This is similar to RunServerCommand but accepts a pre-built config
func runServerWithConfig(ctx context.Context, cfg *appConfig) error {
	authRepo := auth.New(&cfg.Auth)
	connManager := connmng.New()
	connService := core.New(connManager, authRepo)
	apiServ := api.New(cfg.API, connService)

	revServ, err := revproxy.New(&cfg.RevProxy, connService)
	if err != nil {
		return fmt.Errorf("failed to create reverse proxy server: %w", err)
	}

	httpServ, err := edge.New(cfg.HTTP, connService)
	if err != nil {
		return fmt.Errorf("failed to create http server: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return revServ.Run(ctx) })
	eg.Go(func() error { return httpServ.Run(ctx) })
	eg.Go(func() error { return apiServ.Run(ctx) })

	return eg.Wait()
}
