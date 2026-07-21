//go:build windows

package main

import "os"

// hardLinkCount always reports 1 on Windows: os.Lstat's Sys() carries
// Win32FileAttributeData, which has no link count, and querying it would need
// an extra file handle. The symlink and basename guards still apply; the
// hard-link guard is best-effort and Unix-only.
func hardLinkCount(_ os.FileInfo) uint64 { return 1 }
