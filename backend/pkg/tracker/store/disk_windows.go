//go:build windows

package store

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// diskFreeBytes returns the bytes available to the caller in dir's volume via
// GetDiskFreeSpaceExW. The bool is false if the call fails (best-effort).
func diskFreeBytes(dir string) (int64, bool) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var freeAvailable uint64
	r, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeAvailable)),
		0, 0,
	)
	if r == 0 {
		return 0, false
	}
	return int64(freeAvailable), true
}
