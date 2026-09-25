package config

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address        string
	DatabaseURL    string
	FrontendOrigin string
	CookieSecure   bool
	CookieName     string
	SessionTTL     time.Duration
	AutoMigrate    bool
	LogLevel       slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		Address:        env("SILICON_ADDRESS", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		FrontendOrigin: env("SILICON_FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieName:     env("SILICON_SESSION_COOKIE", "silicon_session"),
		SessionTTL:     24 * time.Hour,
		AutoMigrate:    envBool("SILICON_AUTO_MIGRATE", true),
		CookieSecure:   envBool("SILICON_COOKIE_SECURE", false),
		LogLevel:       slog.LevelInfo,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if value := os.Getenv("SILICON_SESSION_TTL"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 15*time.Minute {
			return Config{}, errors.New("SILICON_SESSION_TTL must be a duration of at least 15m")
		}
		cfg.SessionTTL = duration
	}
	switch strings.ToLower(env("SILICON_LOG_LEVEL", "info")) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}
