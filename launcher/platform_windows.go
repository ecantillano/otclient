//go:build windows

package launcher

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32MoveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

func replaceFile(source, destination string) error {
	sourcePointer, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPointer, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	result, _, callErr := kernel32MoveFileEx.Call(
		uintptr(unsafe.Pointer(sourcePointer)),
		uintptr(unsafe.Pointer(destinationPointer)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if result == 0 {
		return callErr
	}
	return nil
}

func syncDirectory(directory string) error {
	// Windows does not permit opening a directory with os.Open for a portable
	// FlushFileBuffers call. MoveFileExW with WRITE_THROUGH above provides the
	// durability guarantee needed by atomic state replacement.
	_, err := os.Stat(directory)
	return err
}
