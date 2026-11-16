package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server-side library search must preserve the browser's Unicode-aware,
// wildcard-literal, HTML-stripped matching. These assert the custom SQLite
// functions that back it.
func TestSQLiteCustomFunctions(t *testing.T) {
	s := newTestStore(t)

	cases := []struct {
		hay, needleLower string
		want             bool
	}{
		{"Login works", "login", true},
		{"Login works", "LOGIN", false}, // caller pre-lowercases the needle
		{"ПРИВЕТ Мир", "мир", true},     // non-Latin case-insensitive (ASCII lower() can't)
		{"Grüße", "grüße", true},
		{"100% done", "100%", true}, // % is literal, not a wildcard
		{"a_b_c", "_", true},        // _ is literal, not a wildcard
		{"nomatch", "zzz", false},
		{"anything", "", true}, // empty needle matches
	}
	for _, c := range cases {
		var got bool
		require.NoError(t, s.db.Raw("SELECT ttgo_icontains(?, ?)", c.hay, c.needleLower).Scan(&got).Error)
		assert.Equalf(t, c.want, got, "icontains(%q, %q)", c.hay, c.needleLower)
	}

	var stripped string
	require.NoError(t, s.db.Raw("SELECT ttgo_striphtml(?)", "<p>a &amp; b&#39;s</p>").Scan(&stripped).Error)
	assert.Equal(t, "a & b's", stripped, "strips tags and decodes entities like the browser")

	var lowered string
	require.NoError(t, s.db.Raw("SELECT ttgo_lower(?)", "ПРИВЕТ").Scan(&lowered).Error)
	assert.Equal(t, "привет", lowered)
}
