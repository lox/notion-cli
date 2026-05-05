//go:build windows

package mcp

import (
	"os"

	"golang.org/x/sys/windows"
)

func acquireFileLock(path string) (*os.File, error) {
	lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	var overlapped windows.Overlapped
	if err := windows.LockFileEx(
		windows.Handle(lockFile.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1,
		0,
		&overlapped,
	); err != nil {
		_ = lockFile.Close()
		return nil, err
	}

	return lockFile, nil
}

func releaseFileLock(lockFile *os.File) error {
	var overlapped windows.Overlapped
	err := windows.UnlockFileEx(windows.Handle(lockFile.Fd()), 0, 1, 0, &overlapped)
	closeErr := lockFile.Close()
	if err != nil {
		return err
	}
	return closeErr
}
