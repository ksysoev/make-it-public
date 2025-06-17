package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWatcher_AddError(t *testing.T) {
	_, err := NewFileWatcher("/non-existent-path-xyz")
	assert.Error(t, err)
}

func TestWatcher_SubscribeAndUnsubscribe(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)

	assert.NoError(t, err)

	defer func() { _ = w.Close() }()

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

	defer func() { _ = w.Close() }()

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

	defer func() { _ = w.Close() }()

	sub := w.Subscribe()
	defer w.Unsubscribe(sub)

	testFile := filepath.Join(tmpDir, "file.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0o600)
	assert.NoError(t, err)

	select {
	case n := <-sub:
		assert.Equal(t, testFile, n.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("Did not receive notification for file write")
	}
}

func TestWatcher_Close(t *testing.T) {
	tmpDir := t.TempDir()
	w, err := NewFileWatcher(tmpDir)
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
}
