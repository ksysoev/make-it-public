package tcpedge

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortPool_AllocateAndRelease(t *testing.T) {
	p := newPortPool(10000, 10009) // 10 ports

	port, err := p.Allocate()

	require.NoError(t, err)
	assert.GreaterOrEqual(t, port, 10000)
	assert.LessOrEqual(t, port, 10009)

	p.Release(port)

	assert.Equal(t, 10, p.Available())
}

func TestPortPool_AllPortsUsed(t *testing.T) {
	const portMin, portMax = 10000, 10002 // 3 ports

	p := newPortPool(portMin, portMax)
	allocated := make([]int, 0, 3)

	for i := range 3 {
		port, err := p.Allocate()
		require.NoError(t, err, "allocation %d should succeed", i)

		allocated = append(allocated, port)
	}

	_, err := p.Allocate()
	assert.ErrorIs(t, err, ErrPortPoolExhausted)

	// Release one and it should become allocatable again.
	p.Release(allocated[0])

	port, err := p.Allocate()
	require.NoError(t, err)
	assert.Equal(t, allocated[0], port)
}

func TestPortPool_Available(t *testing.T) {
	p := newPortPool(10000, 10004) // 5 ports

	assert.Equal(t, 5, p.Available())

	port, err := p.Allocate()
	require.NoError(t, err)
	assert.Equal(t, 4, p.Available())

	p.Release(port)
	assert.Equal(t, 5, p.Available())
}

func TestPortPool_NoDuplicates(t *testing.T) {
	const portMin, portMax = 10000, 10099 // 100 ports

	p := newPortPool(portMin, portMax)
	seen := make(map[int]struct{}, 100)

	for range 100 {
		port, err := p.Allocate()
		require.NoError(t, err)

		_, duplicate := seen[port]
		assert.False(t, duplicate, "duplicate port allocated: %d", port)

		seen[port] = struct{}{}
	}
}

func TestPortPool_ConcurrentAllocations(t *testing.T) {
	const (
		portMin    = 10000
		portMax    = 10199 // 200 ports
		goroutines = 50
	)

	p := newPortPool(portMin, portMax)

	var (
		mu   sync.Mutex
		seen = make(map[int]struct{}, goroutines)
		wg   sync.WaitGroup
	)

	for range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			port, err := p.Allocate()
			require.NoError(t, err)

			mu.Lock()
			_, dup := seen[port]
			assert.False(t, dup, "concurrent duplicate port: %d", port)
			seen[port] = struct{}{}
			mu.Unlock()
		}()
	}

	wg.Wait()
}

func TestPortPool_ReleaseUnknownPort(t *testing.T) {
	// Releasing a port that was never allocated should not panic.
	p := newPortPool(10000, 10009)

	assert.NotPanics(t, func() { p.Release(9999) })
}
