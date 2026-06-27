//go:build !windows

package store

import "syscall"

// diskFreeBytes returns the bytes available to an unprivileged user in dir's
// filesystem. The bool is false if the platform call fails (best-effort).
func diskFreeBytes(dir string) (int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, false
	}
	return int64(stat.Bavail) * int64(stat.Bsize), true
}
