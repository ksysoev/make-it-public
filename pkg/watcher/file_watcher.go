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

//nolint:govet // linter mistakes Mutex to be smaller
type FileWatcher struct {
	mu          sync.Mutex
	wg          sync.WaitGroup
	watcher     *fsnotify.Watcher
	subscribers map[Subscriber]struct{}
	done        chan struct{}
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
		done:        make(chan struct{}),
		wg:          sync.WaitGroup{},
	}

	fw.wg.Add(1)
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
	defer fw.wg.Done()

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
		case <-fw.done:
			return
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
	close(fw.done)

	err := fw.watcher.Close()

	fw.wg.Wait()

	return err
}
