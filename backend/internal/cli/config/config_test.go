package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setIsolatedHome points config path resolution at dir so tests never read or
// write the developer's real ~/.ttgo/config.json. os.UserHomeDir() reads $HOME on
// Unix but %USERPROFILE% on Windows, so set both to stay hermetic on every OS.
func setIsolatedHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestLoadDefault(t *testing.T) {
	dir := t.TempDir()
	setIsolatedHome(t, dir)
	t.Setenv("TTGO_SERVER_URL", "")
	t.Setenv("TTGO_API_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, "http://localhost:8080")
	}
	if cfg.APIToken != "" {
		t.Errorf("APIToken = %q, want empty", cfg.APIToken)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	setIsolatedHome(t, dir)
	t.Setenv("TTGO_SERVER_URL", "")
	t.Setenv("TTGO_API_TOKEN", "")

	cfg := &Config{ServerURL: "http://example.com:9090", APIToken: "test-token-123"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.ServerURL != cfg.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, cfg.ServerURL)
	}
	if loaded.APIToken != cfg.APIToken {
		t.Errorf("APIToken = %q, want %q", loaded.APIToken, cfg.APIToken)
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	setIsolatedHome(t, dir)
	t.Setenv("TTGO_SERVER_URL", "http://env-server:1234")
	t.Setenv("TTGO_API_TOKEN", "env-token-abc")

	cfg := &Config{ServerURL: "http://file-server:5678", APIToken: "file-token-xyz"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.ServerURL != "http://env-server:1234" {
		t.Errorf("ServerURL = %q, want env override", loaded.ServerURL)
	}
	if loaded.APIToken != "env-token-abc" {
		t.Errorf("APIToken = %q, want env override", loaded.APIToken)
	}
}

func TestConfigFilePath(t *testing.T) {
	dir := t.TempDir()
	setIsolatedHome(t, dir)

	cfg := &Config{ServerURL: "http://localhost:8080", APIToken: "tok"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	expectedPath := filepath.Join(dir, ".ttgo", "config.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", expectedPath)
	}
}
