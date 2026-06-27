package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DB_PATH", "")
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_PASSWORD", "")

	cfg := Load()

	assert.Equal(t, "tracker.db", cfg.DBPath)
	assert.Equal(t, "http://localhost:5173", cfg.CORSOrigin)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Empty(t, cfg.AdminEmail)
	assert.Empty(t, cfg.AdminPassword)
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("CORS_ORIGIN", "https://example.com")
	t.Setenv("LISTEN_ADDR", ":9000")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "secret")

	cfg := Load()

	assert.Equal(t, "/tmp/test.db", cfg.DBPath)
	assert.Equal(t, "https://example.com", cfg.CORSOrigin)
	assert.Equal(t, ":9000", cfg.ListenAddr)
	assert.Equal(t, "admin@example.com", cfg.AdminEmail)
	assert.Equal(t, "secret", cfg.AdminPassword)
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TTGO_TEST_KEY_SET", "value")
	assert.Equal(t, "value", envOrDefault("TTGO_TEST_KEY_SET", "fallback"))

	t.Setenv("TTGO_TEST_KEY_EMPTY", "")
	assert.Equal(t, "fallback", envOrDefault("TTGO_TEST_KEY_EMPTY", "fallback"))

	assert.Equal(t, "fallback", envOrDefault("TTGO_TEST_KEY_UNSET", "fallback"))
}
