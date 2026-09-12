//go:build windows

package broker

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// ErrLockHeld reports the broker lockfile is held by another launcher.
var ErrLockHeld = errors.New("broker lock held")

// flockExclusive acquires an exclusive immediate-fail LockFileEx on f.
func flockExclusive(f *os.File) error {
	var ol windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol); err != nil {
		return fmt.Errorf("%w: %v", ErrLockHeld, err)
	}
	return nil
}

// flockUnlock releases the lock on f.
func flockUnlock(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
