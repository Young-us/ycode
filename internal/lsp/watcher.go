package lsp

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileOp represents a file operation
type FileOp int

const (
	FileOpCreate FileOp = iota
	FileOpModify
	FileOpDelete
)

// FileWatcher watches for file changes
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	rootDir  string
	callback func(path string, op FileOp)
	done     chan struct{}
	mu       sync.Mutex
	started  bool
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(rootDir string, callback func(path string, op FileOp)) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		watcher:  watcher,
		rootDir:  rootDir,
		callback: callback,
		done:     make(chan struct{}),
	}, nil
}

// Start starts watching for file changes
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.started {
		return nil
	}

	// Add root directory and subdirectories
	if err := fw.addRecursive(fw.rootDir); err != nil {
		return err
	}

	// Start watching
	go fw.watch()

	fw.started = true
	return nil
}

// Stop stops watching for file changes
func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.started {
		return
	}

	close(fw.done)
	fw.watcher.Close()
	fw.started = false
}

// addRecursive adds a directory and all subdirectories to the watcher
func (fw *FileWatcher) addRecursive(dir string) error {
	// Skip hidden directories and common ignored directories
	base := filepath.Base(dir)
	if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" {
		return nil
	}

	// Add directory to watcher
	if err := fw.watcher.Add(dir); err != nil {
		log.Printf("failed to watch directory %s: %v", dir, err)
		return nil // Continue even if one directory fails
	}

	// Walk subdirectories
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	for _, entry := range entries {
		info, err := os.Stat(entry)
		if err != nil {
			continue
		}

		if info.IsDir() {
			if err := fw.addRecursive(entry); err != nil {
				log.Printf("failed to add recursive watch for %s: %v", entry, err)
			}
		}
	}

	return nil
}

// watch watches for file changes
func (fw *FileWatcher) watch() {
	for {
		select {
		case <-fw.done:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Skip hidden files
			base := filepath.Base(event.Name)
			if strings.HasPrefix(base, ".") {
				continue
			}

			// Determine operation
			var op FileOp
			if event.Has(fsnotify.Create) {
				op = FileOpCreate

				// If it's a directory, add it to the watcher
				if isDir(event.Name) {
					if err := fw.addRecursive(event.Name); err != nil {
						log.Printf("failed to add new directory to watcher: %v", err)
					}
				}
			} else if event.Has(fsnotify.Write) {
				op = FileOpModify
			} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				op = FileOpDelete
			} else {
				continue
			}

			// Call callback
			if fw.callback != nil {
				fw.callback(event.Name, op)
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("file watcher error: %v", err)
		}
	}
}

// isDir checks if a path is a directory
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
