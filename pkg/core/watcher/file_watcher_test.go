package watcher

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
)

type mockWatcher struct {
	Events      chan fsnotify.Event
	Errors      chan error
	CloseCalled bool
}

func (m *mockWatcher) Close() error {
	m.CloseCalled = true
	return nil
}

func TestNewWatcher_AddError(t *testing.T) {
	_, err := NewFileWatcher("/non-existent-path-xyz")
	assert.Error(t, err)
}

func TestWatcher_SubscribeAndUnsubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)
	assert.NoError(t, err)
	defer w.Close()

	sub := w.Subscribe()
	assert.NotNil(t, sub)
	assert.Equal(t, 1, len(w.subscribers))

	w.Unsubscribe(sub)
	assert.Equal(t, 0, len(w.subscribers))
}

func TestWatcher_NotifyAll(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)
	assert.NoError(t, err)
	defer w.Close()

	sub := w.Subscribe()
	defer w.Unsubscribe(sub)

	notification := Notification{Path: "test"}
	w.notifyAll(notification)

	select {
	case n := <-sub:
		assert.Equal(t, notification, n)
	case <-time.After(time.Second):
		t.Fatal("Did not receive notification")
	}
}

func TestWatcher_RunAndEvent(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)
	assert.NoError(t, err)
	defer w.Close()

	sub := w.Subscribe()
	defer w.Unsubscribe(sub)

	testFile := filepath.Join(tmpDir, "file.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	assert.NoError(t, err)

	select {
	case n := <-sub:
		assert.Equal(t, testFile, n.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("Did not receive notification for file write")
	}
}

func TestWatcher_RunError(t *testing.T) {
	mw := &mockWatcher{
		Events: make(chan fsnotify.Event),
		Errors: make(chan error, 1),
	}
	w := &FileWatcher{
		watcher:     nil,
		subscribers: make(map[Subscriber]struct{}),
	}

	w.watcher = &fsnotify.Watcher{
		Events: mw.Events,
		Errors: mw.Errors,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.run()
	}()

	mw.Errors <- errors.New("test error")
	close(mw.Errors)
	close(mw.Events)
	wg.Wait()
}

func TestWatcher_Close(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
}
