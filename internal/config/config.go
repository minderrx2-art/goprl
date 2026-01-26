package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()
	var databaseURL, redisURL, port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}
	if databaseURL = os.Getenv("DATABASE_URL"); databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	if redisURL = os.Getenv("REDIS_URL"); redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}
	return &Config{
		DatabaseURL: databaseURL,
		RedisURL:    redisURL,
		Port:        port,
	}, nil
}
