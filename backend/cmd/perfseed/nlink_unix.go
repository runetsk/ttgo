//go:build !windows

package main

import (
	"os"
	"syscall"
)

// hardLinkCount returns the link count of fi's inode, or 1 when the platform
// stat shape is unavailable. Used to refuse seeding through a hard link that
// aliases a real database.
func hardLinkCount(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Nlink)
	}
	return 1
}
