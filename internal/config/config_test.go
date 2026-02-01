package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	// Helper to clear env
	clearEnv := func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("PORT")
	}

	t.Run("Missing DATABASE_URL", func(t *testing.T) {
		clearEnv()
		os.Setenv("REDIS_URL", "redis://localhost:6379")
		_, err := NewConfig()
		if err == nil {
			t.Error("Expected error when DATABASE_URL is missing")
		}
	})

	t.Run("Missing REDIS_URL", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		_, err := NewConfig()
		if err == nil {
			t.Error("Expected error when REDIS_URL is missing")
		}
	})

	t.Run("Default Port", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("REDIS_URL", "redis://localhost:6379")

		cfg, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if cfg.Port != "8080" {
			t.Errorf("Expected default port 8080, got %s", cfg.Port)
		}
	})

	t.Run("Custom Port", func(t *testing.T) {
		clearEnv()
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("REDIS_URL", "redis://localhost:6379")
		os.Setenv("PORT", "9090")

		cfg, err := NewConfig()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if cfg.Port != "9090" {
			t.Errorf("Expected port 9090, got %s", cfg.Port)
		}
	})
}
