//go:build !linux && !windows

package launcher

func availableDiskBytes(string) (uint64, bool, error) {
	return 0, false, nil
}
