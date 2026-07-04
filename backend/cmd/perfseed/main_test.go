package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRefusesNonPerfDB(t *testing.T) {
	t.Chdir(t.TempDir())
	err := run([]string{"-db", "tracker.db", "-manifest", "m.json"}, &bytes.Buffer{})
	require.ErrorContains(t, err, "refusing to touch")
}

func TestRunRequiresFlags(t *testing.T) {
	t.Chdir(t.TempDir())
	err := run(nil, &bytes.Buffer{})
	require.ErrorContains(t, err, "-db and -manifest are required")
}

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

// Full integration: seeds the small tier into a temp scratch DB (~10k rows,
// a few seconds), writes the manifest, and validates a token end-to-end.
func TestRunSeedsAndWritesManifest(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := t.TempDir()
	db := filepath.Join(dir, "perf-test.db")
	mf := filepath.Join(dir, "manifest.json")

	// Pre-create the manifest world-readable to prove the writer tightens perms
	// on an existing file — os.WriteFile alone does not chmod an existing file,
	// so a stale 0644 manifest would otherwise keep 0644 while holding raw tokens.
	require.NoError(t, os.WriteFile(mf, []byte("{}"), 0o644))

	var out bytes.Buffer
	err := run([]string{"-db", db, "-manifest", mf, "-users", "2", "-tokens", "3", "-wipe"}, &out)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "results=10000")

	raw, err := os.ReadFile(mf)
	require.NoError(t, err)
	var m manifest
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, "small", m.Tier)
	assert.Len(t, m.Tokens, 3)
	assert.Len(t, m.UserEmails, 2)
	require.Len(t, m.IngestTestCases, 500)
	// k6 sends test_name_snapshot straight from the manifest, so the pairs
	// must carry the real seeded names — not leave JS to re-derive the format.
	assert.NotEmpty(t, m.IngestTestCases[0].ID)
	assert.Equal(t, "Ingest TC 0001", m.IngestTestCases[0].Name)

	info, err := os.Stat(mf)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "manifest with raw tokens must be 0600 even if it pre-existed")

	s, err := store.New(db)
	require.NoError(t, err)
	defer s.Close()
	tok, err := s.ValidateToken(m.Tokens[0])
	require.NoError(t, err)
	require.NotNil(t, tok)
	assert.Equal(t, "write", tok.Scope)
}
