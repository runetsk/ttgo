package store

import (
	"database/sql"
	"html"
	"regexp"
	"strings"
	"sync"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// customDriverName is a SQLite driver registered with TTGO's scalar functions,
// used for server-side library filtering. SQLite's built-in lower()/LIKE are
// ASCII-only, so faithful Unicode-aware, wildcard-literal, HTML-stripped matching
// (what the browser does today) needs Go-backed functions.
const customDriverName = "ttgo_sqlite3"

var registerDriverOnce sync.Once

// registerSQLiteDriver registers the custom driver once per process (database/sql
// panics on a duplicate name).
func registerSQLiteDriver() {
	registerDriverOnce.Do(func() {
		sql.Register(customDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if err := conn.RegisterFunc("ttgo_icontains", sqliteIContains, true); err != nil {
					return err
				}
				if err := conn.RegisterFunc("ttgo_lower", sqliteLower, true); err != nil {
					return err
				}
				return conn.RegisterFunc("ttgo_striphtml", sqliteStripHTML, true)
			},
		})
	})
}

// sqliteIContains reports whether needleLower is a Unicode case-insensitive
// substring of haystack. The caller pre-lowercases the needle; the needle is
// matched literally, so % and _ are ordinary characters. An empty needle matches.
func sqliteIContains(haystack, needleLower string) bool {
	if needleLower == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), needleLower)
}

// sqliteLower is Go's Unicode-aware lowercasing, for case-insensitive sort keys.
func sqliteLower(s string) string {
	return strings.ToLower(s)
}

var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

// sqliteStripHTML removes tags and decodes entities, mirroring the frontend's
// DOMParser-based stripHtml so description search matches the same plain text.
func sqliteStripHTML(s string) string {
	return html.UnescapeString(htmlTagRE.ReplaceAllString(s, ""))
}
