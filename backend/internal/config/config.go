package config

import "os"

// Config holds all application-level configuration, loaded once at startup.
type Config struct {
	DBPath        string
	AdminEmail    string
	AdminPassword string
	CORSOrigin    string
	ListenAddr    string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		DBPath:        envOrDefault("DB_PATH", "tracker.db"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		CORSOrigin:    envOrDefault("CORS_ORIGIN", "http://localhost:5173"),
		ListenAddr:    envOrDefault("LISTEN_ADDR", ":8080"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
