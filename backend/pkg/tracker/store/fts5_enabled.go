//go:build sqlite_fts5

package store

// fts5BuildTagSet reports whether the binary was compiled with the sqlite_fts5
// build tag. That tag is required for the SQLite driver (mattn/go-sqlite3) to
// include the FTS5 full-text search module used by search.go. See CLAUDE.md and
// the Makefile (`make setup` / `make build`).
const fts5BuildTagSet = true
