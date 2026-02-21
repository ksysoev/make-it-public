package tcpedge

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
)

// ErrPortPoolExhausted is returned when no ports are available in the configured range.
var ErrPortPoolExhausted = errors.New("no available ports in range")

// portPool manages a set of TCP ports available for dynamic allocation.
// Ports are selected randomly from the configured range.
type portPool struct {
	used map[int]struct{}
	mu   sync.Mutex
	min  int
	max  int
}

// newPortPool creates a portPool for the inclusive range [minPort, maxPort].
func newPortPool(minPort, maxPort int) *portPool {
	return &portPool{
		min:  minPort,
		max:  maxPort,
		used: make(map[int]struct{}),
	}
}

// Allocate picks a random available port from the pool.
// It returns ErrPortPoolExhausted if every port in the range is in use.
func (p *portPool) Allocate() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	size := p.max - p.min + 1
	if len(p.used) >= size {
		return 0, ErrPortPoolExhausted
	}

	// Random probing: fast path for sparse pools.
	const maxProbes = 10

	for range maxProbes {
		port := p.min + rand.IntN(size) //nolint:gosec // non-cryptographic port selection is intentional
		if _, inUse := p.used[port]; !inUse {
			p.used[port] = struct{}{}
			return port, nil
		}
	}

	// Linear fallback: used when random probing keeps colliding (dense pool).
	for port := p.min; port <= p.max; port++ {
		if _, inUse := p.used[port]; !inUse {
			p.used[port] = struct{}{}
			return port, nil
		}
	}

	return 0, fmt.Errorf("pool size=%d used=%d: %w", size, len(p.used), ErrPortPoolExhausted)
}

// Release returns a port to the pool so it can be reused.
func (p *portPool) Release(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.used, port)
}

// Available returns the number of unallocated ports remaining in the pool.
func (p *portPool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return (p.max - p.min + 1) - len(p.used)
}
