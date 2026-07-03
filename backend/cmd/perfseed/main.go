// Command perfseed provisions a scratch TTGO database for performance tests:
// a deterministic dataset (folders/categories/test cases/runs/results), perf
// users, and write-scoped API tokens. It writes a JSON manifest consumed by
// the k6 scenarios under perf/k6/. As a guard, it refuses to touch any
// database whose basename does not match perf-*.db.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
	"ttgo/pkg/tracker/store"
)

type manifest struct {
	Tier              string    `json:"tier"`
	DB                string    `json:"db"`
	SeededAt          time.Time `json:"seeded_at"`
	Tokens            []string  `json:"tokens"`
	UserEmails        []string  `json:"user_emails"`
	IngestTestCaseIDs []string  `json:"ingest_test_case_ids"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("perfseed", flag.ContinueOnError)
	dbPath := fs.String("db", "", "path to the perf scratch database (basename must match perf-*.db)")
	tier := fs.String("tier", "small", "dataset tier (phase 1: small)")
	seed := fs.Uint64("seed", 1, "seed for deterministic data generation")
	users := fs.Int("users", 10, "number of perf users to create")
	tokens := fs.Int("tokens", 100, "number of write-scoped API tokens (keep >= peak VUs so ingest load spreads across token rows rather than hammering last_used_at on a few)")
	password := fs.String("password", "perfseed-local-only", "password shared by perf users")
	manifestPath := fs.String("manifest", "", "path for the k6 seed manifest JSON")
	wipe := fs.Bool("wipe", false, "delete the database (and -wal/-shm siblings) before seeding")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dbPath == "" || *manifestPath == "" {
		return fmt.Errorf("-db and -manifest are required")
	}
	if !store.IsPerfDBPath(*dbPath) {
		return fmt.Errorf("refusing to touch %q: perfseed only operates on databases named perf-*.db", *dbPath)
	}

	absDB, err := filepath.Abs(*dbPath)
	if err != nil {
		return err
	}
	absManifest, err := filepath.Abs(*manifestPath)
	if err != nil {
		return err
	}

	// IsPerfDBPath only checks the basename, so a symlink named perf-*.db could
	// point at a real database. Refuse symlinks (and their -wal/-shm siblings)
	// before we wipe or open anything.
	for _, p := range []string{absDB, absDB + "-wal", absDB + "-shm"} {
		if fi, lerr := os.Lstat(p); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to touch %q: it is a symlink; perfseed only operates on regular scratch files", p)
		}
	}

	if *wipe {
		for _, p := range []string{absDB, absDB + "-wal", absDB + "-shm"} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("wipe %s: %w", p, err)
			}
		}
	}

	// store.New creates backups/ and secret.key relative to the CWD; chdir so
	// that clutter lands next to the scratch DB, not wherever we were invoked.
	if err := os.MkdirAll(filepath.Dir(absDB), 0o755); err != nil {
		return err
	}
	if err := os.Chdir(filepath.Dir(absDB)); err != nil {
		return err
	}

	cfg, err := store.PerfTier(*tier)
	if err != nil {
		return err
	}
	cfg.Seed = *seed

	s, err := store.New(absDB)
	if err != nil {
		return err
	}
	defer s.Close()

	res, err := s.SeedPerfDataset(cfg)
	if err != nil {
		return fmt.Errorf("seed dataset: %w", err)
	}
	principals, err := s.SeedPerfPrincipals(*users, *tokens, *password)
	if err != nil {
		return fmt.Errorf("seed principals: %w", err)
	}

	m := manifest{
		Tier: *tier, DB: absDB, SeededAt: time.Now(),
		Tokens: principals.Tokens, UserEmails: principals.UserEmails,
		IngestTestCaseIDs: res.IngestPoolCaseIDs,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// 0600: the manifest holds raw bearer tokens. WriteFile does not tighten
	// permissions on an already-existing file, so chmod explicitly to ensure a
	// stale, looser manifest cannot keep world-readable perms after a rewrite.
	if err := os.WriteFile(absManifest, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(absManifest, 0o600); err != nil {
		return err
	}

	fmt.Fprintf(out,
		"seeded %s tier=%s: folders=%d categories=%d cases=%d (+%d ingest-pool) runs=%d results=%d users=%d tokens=%d\nmanifest: %s\n",
		absDB, *tier, res.Folders, res.Categories, res.TestCases,
		len(res.IngestPoolCaseIDs), res.TestRuns, res.RunResults,
		len(principals.UserEmails), len(principals.Tokens), absManifest)
	return nil
}
