//go:build windows

package launcher

import (
	"syscall"
	"unsafe"
)

var kernel32DiskFree = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func availableDiskBytes(path string) (uint64, bool, error) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, true, err
	}
	var available uint64
	result, _, callErr := kernel32DiskFree.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		uintptr(unsafe.Pointer(&available)),
		0,
		0,
	)
	if result == 0 {
		return 0, true, callErr
	}
	return available, true, nil
}
