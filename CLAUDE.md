# TTGO Development Guidelines

Last updated: 2026-07-28

## Project Structure

```
backend/          Go backend (module: ttgo)
  cmd/server/     API server entry point
  cmd/ttgo/       CLI entry point (Cobra)
  internal/
    api/          HTTP handlers & routing (one package per feature)
    cli/          CLI commands
    config/       Configuration loading
    importparser/ Import utilities
    logging/      Logger setup
    ratelimit/    Per-IP / per-token rate limiting
    safehttp/     SSRF-guarded outbound HTTP clients
  pkg/tracker/
    failureanalysis/  AI failure-analysis grouping + worker
    llm/              LLM provider clients
    models/           Domain models
    secretbox/        At-rest secret encryption (AES-256-GCM)
    store/            GORM persistence layer
  docs/           Swagger-generated docs

frontend/         React frontend (Vite)
  src/
    components/   Shared UI components
    pages/        Route-level page components
    contexts/     React contexts
    hooks/        Custom hooks
    utils/        Utility functions
    api.js        Axios API client
  e2e/            Playwright suite + TTGO self-reporting reporter

specs/            Feature specification docs (gitignored, local only)
docs/             Project-level docs (gitignored except docs/images/)
```

## Stack

**Backend** — Go 1.25.6
- `net/http` + `go-pkgz/routegroup` for routing
- GORM + `gorm.io/driver/sqlite` (CGO; wraps `mattn/go-sqlite3`) — requires the `sqlite_fts5` build tag for full-text search
- `golang.org/x/crypto/bcrypt` for auth
- `gorilla/websocket` for real-time
- `microcosm-cc/bluemonday` for HTML sanitization
- `swaggo/swag` + `swaggo/http-swagger` for Swagger docs
- spf13/cobra for the ttgo CLI

**Frontend** — React 19.2 + Vite
- React Router v7, Axios
- Tiptap (rich text editor)
- @dnd-kit (drag and drop)
- Recharts (analytics charts)
- `diff` (jsdiff) for version diffing

## Commands

**Backend** — needs CGO + a C compiler (gcc) and the `sqlite_fts5` build tag (the SQLite driver only compiles FTS5 in with that tag).
```bash
cd backend
make setup     # one-time per machine: sets GOFLAGS=-tags=sqlite_fts5
go build ./...   # works after `make setup`
go test ./...
make build     # or use the Make targets, which carry the tag explicitly
make test
make swagger   # regenerate Swagger docs
```
Without `make setup` (or an explicit `-tags sqlite_fts5`), bare `go` commands still compile but fail at runtime with `no such module: fts5`.

**Frontend**
```bash
cd frontend
npm run dev       # dev server
npm run build     # production build
npm run lint      # ESLint
npm run test:unit # node --test over e2e/reporters/*.test.js and src/**/*.test.js
npm run test:e2e  # Playwright; needs a running stack (see e2e/config.js)
```

## Code Style

- Go: standard conventions (`gofmt`, idiomatic error handling)
- JavaScript/JSX: ESLint enforced; functional React components only
- No new Go modules or npm packages without a clear need

## Testing seams

There is **no component-render harness** in the frontend (no jsdom, no Testing Library) —
`npm run test:unit` is bare `node --test`. So page logic that needs covering goes in a
DOM-free `src/utils/*.js` module with a sibling `*.test.js`, and the `.jsx` file only
places what the helper derived. Anything that genuinely needs a rendered DOM is covered
by Playwright instead.

## Domain Conventions

**Defect status: three values stored, four displayed.** `Defect.Status` is `open | fixed | closed`.
"Needs triage" and "In progress" are *not* stored — both are `open`, split on whether `assignee_id`
is null, because assignment is the triage action. Do not add a fourth stored value to express them.
`"open"` must stay writable (CLI, seed data and external scripts depend on it), and every defect
counter buckets `closed` against everything else, so `fixed` counts as open everywhere. The
derivation lives in `frontend/src/utils/defectQueue.js` (`deriveStatus`).

`test_cases.reverification_flagged` means *every linked defect is fixed-or-closed* — the test is
ready to be retested (`store/defects.go`, `recomputeReverification`).

## Versioning

- Semantic Versioning (`MAJOR.MINOR.PATCH`); the canonical version is the git tag `vX.Y.Z` (it also serves the Go module).
- Record notable changes in `CHANGELOG.md` ([Keep a Changelog](https://keepachangelog.com) format) under `## [Unreleased]` as work lands.
- On release: move the `Unreleased` items under a new `## [X.Y.Z] - YYYY-MM-DD` heading, bump `frontend/package.json` `version`, commit, then tag `vX.Y.Z` on `main`.
- Commit messages follow Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
