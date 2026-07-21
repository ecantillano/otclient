//go:build !windows

package launcher

import (
	"errors"
	"os"
	"syscall"
)

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}

func syncDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
		return err
	}
	return nil
}
