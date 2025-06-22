package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrSiteLocked = fmt.Errorf("site directory is already locked")

type SiteLock struct {
	sync.Locker
	path   string
	locked bool
	mu     sync.Mutex
}

func newSiteLock(path string) *SiteLock {
	return &SiteLock{
		mu:     sync.Mutex{},
		locked: false,
		path:   filepath.Join(path, "site.lock"),
	}
}

func (sl *SiteLock) Lock() error {
	// take complete ownership of the struct
	sl.mu.Lock()
	defer sl.mu.Unlock()

	lockFile, err := os.OpenFile(sl.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return ErrSiteLocked
		}
		return fmt.Errorf("could not create lock file %s: %w", sl.path, err)
	}

	err = lockFile.Close()
	if err != nil {
		return fmt.Errorf("could not close lock file %s: %w", sl.path, err)
	}

	sl.locked = true
	return nil
}

func (sl *SiteLock) Unlock() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if !sl.locked {
		return fmt.Errorf("site lock is not active")
	}

	err := os.Remove(sl.path)
	if err != nil {
		if os.IsNotExist(err) {
			sl.locked = false
			return nil // lock file already removed
		}
		return fmt.Errorf("could not remove lock file %s: %w", sl.path, err)
	}

	sl.locked = false
	return nil
}
