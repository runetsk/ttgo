package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL string `json:"server_url"`
	APIToken  string `json:"api_token"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttgo"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads config from ~/.ttgo/config.json, then applies env var overrides.
// Returns defaults if the file doesn't exist.
func Load() (*Config, error) {
	cfg := &Config{
		ServerURL: "http://localhost:8080",
	}

	path, err := configPath()
	if err != nil {
		applyEnvOverrides(cfg)
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// Save writes config to ~/.ttgo/config.json.
func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, data, 0600)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TTGO_SERVER_URL"); v != "" {
		cfg.ServerURL = v
	}
	if v := os.Getenv("TTGO_API_TOKEN"); v != "" {
		cfg.APIToken = v
	}
}
