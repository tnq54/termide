//go:build linux

package server

import (
	"fmt"
	"os"
	"syscall"
)

type databaseLock struct{ file *os.File }

func AcquireDatabaseLock(path string) (interface{ Close() error }, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("database lock is already held: %w", err)
	}
	return databaseLock{file: file}, nil
}

func (l databaseLock) Close() error {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
