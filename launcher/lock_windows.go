//go:build windows

package launcher

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	ErrLocked          = errors.New("launcher or launcher-managed client is already running")
	kernel32LockFile   = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	kernel32UnlockFile = syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")
)

const (
	lockFileExclusive     = 0x2
	lockFileFailImmediate = 0x1
	errorLockViolation    = syscall.Errno(33)
)

type FileLock struct {
	file       *os.File
	overlapped syscall.Overlapped
}

func AcquireFileLock(fileName string) (*FileLock, error) {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	lock := &FileLock{file: file}
	result, _, callErr := kernel32LockFile.Call(
		file.Fd(), lockFileExclusive|lockFileFailImmediate, 0, 1, 0,
		uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	if result == 0 {
		_ = file.Close()
		if errors.Is(callErr, errorLockViolation) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock launcher: %w", callErr)
	}
	if err := writeLockPID(file); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}

func (lock *FileLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	result, _, callErr := kernel32UnlockFile.Call(
		lock.file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&lock.overlapped)),
	)
	closeErr := lock.file.Close()
	lock.file = nil
	if result == 0 {
		return callErr
	}
	return closeErr
}
