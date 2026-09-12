//go:build !windows

package broker

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLockHeld reports the broker lockfile is held by another launcher.
var ErrLockHeld = errors.New("broker lock held")

// flockExclusive acquires an exclusive non-blocking flock on f.
func flockExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("%w: %v", ErrLockHeld, err)
		}
		return fmt.Errorf("flock: %w", err)
	}
	return nil
}

// flockUnlock releases the flock on f.
func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
