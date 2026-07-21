package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

	// Read scenarios GET /api/runs/{id} using these; small tier has 50 runs.
	require.Len(t, m.HistoricalRunIDs, 50)
	assert.NotEmpty(t, m.HistoricalRunIDs[0])

	// The S4 WebSocket scenario logs perf users in for session cookies
	// (Bearer tokens cannot open WS connections), so the manifest must
	// carry the shared password alongside the emails.
	assert.Equal(t, "perfseed-local-only", m.UserPassword)

	info, err := os.Stat(mf)
	require.NoError(t, err)
	if runtime.GOOS != "windows" { // Windows has no POSIX perms; Chmod only toggles read-only
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "manifest with raw tokens must be 0600 even if it pre-existed")
	}

	s, err := store.New(db)
	require.NoError(t, err)
	defer s.Close()
	tok, err := s.ValidateToken(m.Tokens[0])
	require.NoError(t, err)
	require.NotNil(t, tok)
	assert.Equal(t, "write", tok.Scope)
}
