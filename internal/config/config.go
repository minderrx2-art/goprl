package config

import (
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

func NewConfig() (*Config, error) {
	_ = godotenv.Load()
	var databaseURL, redisURL, port, baseURL, rateLimit, env string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}
	if databaseURL = os.Getenv("DATABASE_URL"); databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	if redisURL = os.Getenv("REDIS_URL"); redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}
	if baseURL = os.Getenv("BASE_URL"); baseURL == "" {
		baseURL = "http://localhost:" + port
	}
	if rateLimit = os.Getenv("RATE_LIMIT"); rateLimit == "" {
		rateLimit = "20"
	}
	limit, err := strconv.Atoi(rateLimit)
	if err != nil {
		return nil, fmt.Errorf("RATE_LIMIT is not a valid integer")
	}
	if env = os.Getenv("ENV"); env == "" {
		env = "dev"
	}
	return &Config{
		DatabaseURL: databaseURL,
		RedisURL:    redisURL,
		Port:        port,
		BaseURL:     baseURL,
		RateLimit:   limit,
		Env:         env,
	}, nil
}
