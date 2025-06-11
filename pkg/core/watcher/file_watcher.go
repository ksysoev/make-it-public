package watcher

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Notification struct {
	Path string
}

type Subscriber chan Notification

type FileWatcher struct {
	watcher     *fsnotify.Watcher
	subscribers map[Subscriber]struct{}
	mu          sync.Mutex
}

func NewFileWatcher(paths ...string) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if err := w.Add(path); err != nil {
			w.Close()
			return nil, err
		}
	}
	fw := &FileWatcher{
		watcher:     w,
		subscribers: make(map[Subscriber]struct{}),
		mu:          sync.Mutex{},
	}
	go fw.run()
	return fw, nil
}

func (fw *FileWatcher) Subscribe() Subscriber {
	ch := make(Subscriber, 1)
	fw.mu.Lock()
	fw.subscribers[ch] = struct{}{}
	fw.mu.Unlock()
	return ch
}

func (fw *FileWatcher) Unsubscribe(ch Subscriber) {
	fw.mu.Lock()
	delete(fw.subscribers, ch)
	close(ch)
	fw.mu.Unlock()
}

func (fw *FileWatcher) run() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				fw.notifyAll(Notification{Path: event.Name})
			}
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error(fmt.Sprintf("Watcher error: %v", err))
		}
	}
}

func (fw *FileWatcher) notifyAll(n Notification) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for ch := range fw.subscribers {
		select {
		case ch <- n:
		default:
		}
	}
}

func (fw *FileWatcher) Close() error {
	return fw.watcher.Close()
}
