package conn

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnReq_ID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	connReq := NewRequest(ctx)

	assert.NotZero(t, connReq.ID())
	assert.IsType(t, uuid.UUID{}, connReq.ID())
}

func TestConnReq_ParentContext(t *testing.T) {
	t.Parallel()

	connReq := NewRequest(t.Context())

	assert.Equal(t, t.Context(), connReq.ParentContext())
}

func TestConnReq_WaitConn(t *testing.T) {
	tests := []struct {
		expectErr  error
		name       string
		parentDone bool
		childDone  bool
		sendConn   bool
	}{
		{name: "connection received", parentDone: false, childDone: false, sendConn: true, expectErr: nil},
		{name: "parent context canceled", parentDone: true, childDone: false, sendConn: false, expectErr: errors.New("parent context is canceled")},
		{name: "child context canceled", parentDone: false, childDone: true, sendConn: false, expectErr: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			parentCtx, parentCancel := context.WithCancel(ctx)
			defer parentCancel()

			connReq := NewRequest(parentCtx)

			if tt.parentDone {
				parentCancel()
			}

			childCtx, childCancel := context.WithCancel(ctx)
			defer childCancel()

			if tt.childDone {
				childCancel()
			}

			if tt.sendConn {
				conn := &net.IPConn{}
				go func() {
					time.Sleep(10 * time.Millisecond)
					connReq.SendConn(ctx, conn)
				}()
			}

			conn, err := connReq.WaitConn(childCtx)
			if tt.expectErr != nil {
				assert.ErrorContains(t, err, tt.expectErr.Error())
				assert.Nil(t, conn)

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, conn)
		})
	}
}

func TestConnReq_SendConn(t *testing.T) {
	tests := []struct {
		name       string
		parentDone bool
		childDone  bool
	}{
		{"connection sent", false, false},
		{"parent context canceled", true, false},
		{"child context canceled", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			parentCtx, parentCancel := context.WithCancel(ctx)
			defer parentCancel()

			connReq := NewRequest(parentCtx)

			if tt.parentDone {
				parentCancel()
			}

			childCtx, childCancel := context.WithCancel(ctx)
			defer childCancel()

			if tt.childDone {
				childCancel()
			}

			conn := &net.IPConn{}
			done := make(chan struct{})

			go func() {
				connReq.SendConn(childCtx, conn)
				close(done)
			}()

			if tt.parentDone || tt.childDone {
				select {
				case <-done:
					// This should happen if the parent or child context is canceled
				case <-time.After(100 * time.Millisecond):
					assert.Fail(t, "SendConn should not have completed")
				}
			} else {
				select {
				case c, ok := <-connReq.ch:
					assert.True(t, ok)
					assert.Equal(t, conn, c)
				case <-time.After(100 * time.Millisecond):
					assert.Fail(t, "SendConn did not complete as expected")
				}
			}
		})
	}
}

func TestConnReq_Cancel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	connReq := NewRequest(ctx)

	go func() {
		connReq.Cancel()
	}()

	select {
	case _, ok := <-connReq.ch:
		assert.False(t, ok)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel was not closed as expected")
	}
}

func TestNewConnReq(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	connReq := NewRequest(ctx)

	require.NotNil(t, connReq)
	assert.Equal(t, ctx, connReq.ctx)
	assert.NotNil(t, connReq.ch)
	assert.IsType(t, uuid.UUID{}, connReq.id)
}
