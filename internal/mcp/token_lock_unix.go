//go:build !windows

package mcp

import (
	"os"

	"golang.org/x/sys/unix"
)

func acquireFileLock(path string) (*os.File, error) {
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		_ = lockFile.Close()
		return nil, err
	}
	return lockFile, nil
}

func releaseFileLock(lockFile *os.File) error {
	err := unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
	closeErr := lockFile.Close()
	if err != nil {
		return err
	}
	return closeErr
}
