# TTGO Development Guidelines

Last updated: 2026-04-08

## Project Structure

```
backend/          Go backend (module: ttgo)
  cmd/server/     Entry point
  internal/
    api/          HTTP handlers & routing
    config/       Configuration loading
    importparser/ Import utilities
    logging/      Logger setup
  pkg/tracker/    Core domain models & logic
  docs/           Swagger-generated docs

frontend/         React frontend (Vite)
  src/
    components/   Shared UI components
    pages/        Route-level page components
    contexts/     React contexts
    hooks/        Custom hooks
    utils/        Utility functions
    api.js        Axios API client

specs/            Feature specification docs
docs/             Project-level docs
```

## Stack

**Backend** — Go 1.25.6
- `net/http` + `go-pkgz/routegroup` for routing
- GORM + `gorm.io/driver/sqlite` (CGO; wraps `mattn/go-sqlite3`) — requires the `sqlite_fts5` build tag for full-text search
- `golang.org/x/crypto/bcrypt` for auth
- `gorilla/websocket` for real-time
- `microcosm-cc/bluemonday` for HTML sanitization
- `swaggo/swag` + `swaggo/http-swagger` for Swagger docs

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
npm run dev    # dev server
npm run build  # production build
npm run lint   # ESLint
```

## Code Style

- Go: standard conventions (`gofmt`, idiomatic error handling)
- JavaScript/JSX: ESLint enforced; functional React components only
- No new Go modules or npm packages without a clear need
