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

type Watcher struct {
	watcher     *fsnotify.Watcher
	subscribers map[Subscriber]struct{}
	mu          sync.Mutex
}

func NewWatcher(paths ...string) (*Watcher, error) {
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
	cw := &Watcher{
		watcher:     w,
		subscribers: make(map[Subscriber]struct{}),
	}
	go cw.run()
	return cw, nil
}

func (cw *Watcher) Subscribe() Subscriber {
	ch := make(Subscriber, 1)
	cw.mu.Lock()
	cw.subscribers[ch] = struct{}{}
	cw.mu.Unlock()
	return ch
}

func (cw *Watcher) Unsubscribe(ch Subscriber) {
	cw.mu.Lock()
	delete(cw.subscribers, ch)
	close(ch)
	cw.mu.Unlock()
}

func (cw *Watcher) run() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				cw.notifyAll(Notification{Path: event.Name})
			}
		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			slog.Error(fmt.Sprintf("Watcher error: %v", err))
		}
	}
}

func (cw *Watcher) notifyAll(n Notification) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	for ch := range cw.subscribers {
		select {
		case ch <- n:
		default:
		}
	}
}

func (cw *Watcher) Close() error {
	return cw.watcher.Close()
}
