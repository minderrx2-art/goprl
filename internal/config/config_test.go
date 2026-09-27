package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	clearEnv := func(t *testing.T) {
		t.Helper()
		for _, key := range []string{"DATABASE_URL", "REDIS_URL", "PORT", "BASE_URL", "RATE_LIMIT", "ENV"} {
			t.Setenv(key, "")
		}
	}

	t.Run("Missing DATABASE_URL", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("REDIS_URL", "redis://localhost:6379")
		_, err := Load()
		if err == nil {
			t.Error("Expected error when DATABASE_URL is missing")
		}
	})

	t.Run("Missing REDIS_URL", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		_, err := Load()
		if err == nil {
			t.Error("Expected error when REDIS_URL is missing")
		}
	})

	t.Run("Default Port", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		t.Setenv("REDIS_URL", "redis://localhost:6379")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if cfg.Port != "8080" {
			t.Errorf("Expected default port 8080, got %s", cfg.Port)
		}
	})

	t.Run("Custom Port", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		t.Setenv("REDIS_URL", "redis://localhost:6379")
		t.Setenv("PORT", "9090")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if cfg.Port != "9090" {
			t.Errorf("Expected port 9090, got %s", cfg.Port)
		}
	})
}
