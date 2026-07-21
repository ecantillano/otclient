//go:build linux

package launcher

import "syscall"

func availableDiskBytes(path string) (uint64, bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, true, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), true, nil
}
