package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
	BaseURL     string
	RateLimit   int
	Env         string
}

// Load reads environment variables, loading .env without overriding the environment.
func Load() (*Config, error) {
	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return nil, errors.New("REDIS_URL is not set")
	}
	port := envOrDefault("PORT", "8080")
	limit, err := strconv.Atoi(envOrDefault("RATE_LIMIT", "20"))
	if err != nil {
		return nil, fmt.Errorf("RATE_LIMIT is not a valid integer: %w", err)
	}
	return &Config{
		DatabaseURL: databaseURL,
		RedisURL:    redisURL,
		Port:        port,
		BaseURL:     envOrDefault("BASE_URL", "http://localhost:"+port),
		RateLimit:   limit,
		Env:         envOrDefault("ENV", "dev"),
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
