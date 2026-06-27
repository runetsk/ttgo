.PHONY: swagger build test cli install-cli setup

swagger:
	$(shell go env GOPATH)/bin/swag init \
	  -g internal/api/server.go \
	  -d ./,./internal/api,./pkg/tracker/models \
	  --output ./docs \
	  --parseDependency \
	  --parseInternal

build:
	go build -tags sqlite_fts5 ./...

test:
	go test -tags sqlite_fts5 ./...

cli:
	go build -tags sqlite_fts5 -o ttgo ./cmd/ttgo/

install-cli:
	go install -tags sqlite_fts5 ./cmd/ttgo/

# One-time per machine: make the sqlite_fts5 build tag apply to every `go`
# command (and gopls), so bare `go build`/`go test`/`go run` and the IDE work
# without remembering the tag. Writes GOFLAGS to your user-level `go env`.
setup:
	go env -w GOFLAGS=-tags=sqlite_fts5
