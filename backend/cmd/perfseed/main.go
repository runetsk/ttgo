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
	Profile          string           `json:"profile"`
	Tier             string           `json:"tier,omitempty"`
	DB               string           `json:"db"`
	SeededAt         time.Time        `json:"seeded_at"`
	Tokens           []string         `json:"tokens"`
	UserEmails       []string         `json:"user_emails"`
	UserPassword     string           `json:"user_password"`
	IngestTestCases  []store.PerfCase `json:"ingest_test_cases,omitempty"`
	HistoricalRunIDs []string         `json:"historical_run_ids,omitempty"`

	// ai profile only: newest-first run ids plus the planted-failure answer key
	// for grading real-LLM verdicts against known root causes.
	LatestRunID string                    `json:"latest_run_id,omitempty"`
	RunIDs      []string                  `json:"run_ids,omitempty"`
	GroundTruth []store.AISeedGroundTruth `json:"ground_truth,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("perfseed", flag.ContinueOnError)
	dbPath := fs.String("db", "", "path to the perf scratch database (basename must match perf-*.db)")
	profile := fs.String("profile", "perf", "dataset profile: perf (k6 load-test dataset) or ai (AI failure-analysis demo dataset with realistic failures + ground truth)")
	tier := fs.String("tier", "small", "perf profile only — dataset tier: small (~10k results), medium (~100k), large (~1M)")
	days := fs.Int("days", 30, "ai profile only — daily runs to generate (newest = today)")
	perRun := fs.Int("per-run", 500, "ai profile only — results per run")
	nCases := fs.Int("cases", 600, "ai profile only — test-case catalog size")
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
	if *profile != "perf" && *profile != "ai" {
		return fmt.Errorf("unknown profile %q (perf, ai)", *profile)
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

	// IsPerfDBPath only checks the basename, so indirection could still aim the
	// seeder at a real database or redirect the token manifest. Before wiping,
	// opening, or writing anything, refuse for the DB, its -wal/-shm siblings,
	// AND the manifest: symlinks (WriteFile/Chmod follow them), hard links
	// (Lstat reports a regular file but the inode is shared), and a symlinked
	// immediate parent directory (which relocates every per-file check here;
	// deeper ancestors like macOS's /var stay allowed).
	for _, p := range []string{absDB, absDB + "-wal", absDB + "-shm", absManifest} {
		fi, lerr := os.Lstat(p)
		if lerr != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to touch %q: it is a symlink; perfseed only operates on regular scratch files", p)
		}
		if n := hardLinkCount(fi); n > 1 {
			return fmt.Errorf("refusing to touch %q: it is a hard link (%d links); perfseed only operates on regular scratch files", p, n)
		}
	}
	for _, d := range []string{filepath.Dir(absDB), filepath.Dir(absManifest)} {
		if fi, lerr := os.Lstat(d); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to use directory %q: it is a symlink", d)
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

	s, err := store.New(absDB)
	if err != nil {
		return err
	}
	defer s.Close()

	var m manifest
	var summary string
	switch *profile {
	case "perf":
		cfg, err := store.PerfTier(*tier)
		if err != nil {
			return err
		}
		cfg.Seed = *seed
		res, err := s.SeedPerfDataset(cfg)
		if err != nil {
			return fmt.Errorf("seed dataset: %w", err)
		}
		m = manifest{
			Profile: "perf", Tier: *tier,
			IngestTestCases:  res.IngestPool,
			HistoricalRunIDs: res.HistoricalRunIDs,
		}
		summary = fmt.Sprintf(
			"seeded %s tier=%s: folders=%d categories=%d cases=%d (+%d ingest-pool) runs=%d results=%d",
			absDB, *tier, res.Folders, res.Categories, res.TestCases,
			len(res.IngestPool), res.TestRuns, res.RunResults)
	case "ai":
		cfg := store.DefaultAISeedConfig()
		cfg.Seed, cfg.Days, cfg.ResultsPerRun, cfg.TestCases = *seed, *days, *perRun, *nCases
		res, err := s.SeedAIFailureDataset(cfg)
		if err != nil {
			return fmt.Errorf("seed ai dataset: %w", err)
		}
		m = manifest{
			Profile: "ai", LatestRunID: res.LatestRunID,
			RunIDs: res.RunIDs, GroundTruth: res.GroundTruth,
		}
		summary = fmt.Sprintf(
			"seeded %s profile=ai: runs=%d results=%d failing=%d human-labeled=%d cases=%d\nlatest run (analyze this one): %s\nplanted failure groups:",
			absDB, res.TestRuns, res.RunResults, res.FailingRows, res.LabeledRows, res.TestCases, res.LatestRunID)
		for _, g := range res.GroundTruth {
			summary += fmt.Sprintf(
				"\n  %-26s %-15s expect %s/%s  rows=%d (latest run %d)",
				g.TemplateKey, "["+g.Scenario+"]", g.ExpectedVerdict, g.ExpectedDefect, g.TotalRows, g.LatestRunRows)
		}
	}

	principals, err := s.SeedPerfPrincipals(*users, *tokens, *password)
	if err != nil {
		return fmt.Errorf("seed principals: %w", err)
	}
	m.DB, m.SeededAt = absDB, time.Now()
	m.Tokens, m.UserEmails, m.UserPassword = principals.Tokens, principals.UserEmails, *password
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

	fmt.Fprintf(out, "%s\nusers=%d tokens=%d\nmanifest: %s\n",
		summary, len(principals.UserEmails), len(principals.Tokens), absManifest)
	return nil
}
