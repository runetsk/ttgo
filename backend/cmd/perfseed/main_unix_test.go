//go:build !windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// These guards depend on Unix filesystem semantics: symlink creation is
// unprivileged and Lstat exposes inode link counts. On Windows os.Symlink
// needs Developer Mode and hardLinkCount is a stub, so the tests live here.

// A symlink named perf-*.db passes the basename guard but could point at a real
// database; the CLI must refuse it before touching anything.
func TestRunRefusesSymlinkDB(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-tracker.db")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))
	link := filepath.Join(dir, "perf-evil.db")
	require.NoError(t, os.Symlink(target, link))

	err := run([]string{"-db", link, "-manifest", filepath.Join(dir, "m.json")}, &bytes.Buffer{})
	require.ErrorContains(t, err, "symlink")
}

// A hard link named perf-*.db shares the target's inode; Lstat reports a
// regular file, so the symlink check alone would let a no-wipe run seed
// straight into a real database. The guard must refuse link count > 1.
func TestRunRefusesHardLinkDB(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-tracker.db")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))
	link := filepath.Join(dir, "perf-evil.db")
	require.NoError(t, os.Link(target, link))

	err := run([]string{"-db", link, "-manifest", filepath.Join(dir, "m.json")}, &bytes.Buffer{})
	require.ErrorContains(t, err, "hard link")
}

// os.WriteFile and os.Chmod follow symlinks, so a symlinked manifest would
// redirect raw bearer-token JSON over an arbitrary user-writable file.
func TestRunRefusesSymlinkManifest(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "victim.json")
	require.NoError(t, os.WriteFile(target, []byte("{}"), 0o600))
	link := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.Symlink(target, link))

	err := run([]string{"-db", filepath.Join(dir, "perf-x.db"), "-manifest", link}, &bytes.Buffer{})
	require.ErrorContains(t, err, "symlink")
}

// A symlinked scratch directory relocates every per-file check above it, so
// the immediate parent of the DB (and manifest) must not be a symlink.
func TestRunRefusesSymlinkParentDir(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real-data")
	require.NoError(t, os.Mkdir(real, 0o755))
	scratch := filepath.Join(dir, "scratch")
	require.NoError(t, os.Symlink(real, scratch))

	err := run([]string{"-db", filepath.Join(scratch, "perf-x.db"), "-manifest", filepath.Join(dir, "m.json")}, &bytes.Buffer{})
	require.ErrorContains(t, err, "symlink")
}
