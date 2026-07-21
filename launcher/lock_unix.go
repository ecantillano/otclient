//go:build !windows

package launcher

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

var ErrLocked = errors.New("launcher or launcher-managed client is already running")

type FileLock struct {
	file *os.File
}

func AcquireFileLock(fileName string) (*FileLock, error) {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock launcher: %w", err)
	}
	if err := writeLockPID(file); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	return &FileLock{file: file}, nil
}

func (lock *FileLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	closeErr := lock.file.Close()
	lock.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
