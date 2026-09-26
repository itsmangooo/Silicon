package config

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                 string
	DatabaseURL             string
	FrontendOrigin          string
	CookieSecure            bool
	CookieName              string
	SessionTTL              time.Duration
	AutoMigrate             bool
	LogLevel                slog.Level
	EncryptionKey           []byte
	GitHubAppID             int64
	GitHubPrivateKey        string
	GitHubWebhookSecret     string
	GitHubAPIURL            string
	CloudflareAPIURL        string
	LocalDockerEnabled      bool
	DockerBinary            string
	RuntimeLogFollowTimeout time.Duration
	PublicURL               string
	TrustForwardedProto     bool
}

func Load() (Config, error) {
	cfg := Config{
		Address:                 env("SILICON_ADDRESS", ":8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		FrontendOrigin:          env("SILICON_FRONTEND_ORIGIN", "http://localhost:5173"),
		CookieName:              env("SILICON_SESSION_COOKIE", "silicon_session"),
		SessionTTL:              24 * time.Hour,
		AutoMigrate:             envBool("SILICON_AUTO_MIGRATE", true),
		CookieSecure:            envBool("SILICON_COOKIE_SECURE", false),
		LogLevel:                slog.LevelInfo,
		GitHubPrivateKey:        os.Getenv("SILICON_GITHUB_PRIVATE_KEY"),
		GitHubWebhookSecret:     os.Getenv("SILICON_GITHUB_WEBHOOK_SECRET"),
		GitHubAPIURL:            env("SILICON_GITHUB_API_URL", "https://api.github.com"),
		CloudflareAPIURL:        env("SILICON_CLOUDFLARE_API_URL", "https://api.cloudflare.com/client/v4"),
		LocalDockerEnabled:      envBool("SILICON_LOCAL_DOCKER_ENABLED", false),
		DockerBinary:            env("SILICON_DOCKER_BINARY", "docker"),
		RuntimeLogFollowTimeout: 5 * time.Minute,
		PublicURL:               strings.TrimRight(env("SILICON_PUBLIC_URL", "http://localhost:5173"), "/"),
		TrustForwardedProto:     envBool("SILICON_TRUST_FORWARDED_PROTO", false),
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
	if value := os.Getenv("SILICON_RUNTIME_LOG_FOLLOW_TIMEOUT"); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 10*time.Second || duration > time.Hour {
			return Config{}, errors.New("SILICON_RUNTIME_LOG_FOLLOW_TIMEOUT must be between 10s and 1h")
		}
		cfg.RuntimeLogFollowTimeout = duration
	}
	if value := os.Getenv("SILICON_ENCRYPTION_KEY"); value != "" {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 {
			return Config{}, errors.New("SILICON_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
		}
		cfg.EncryptionKey = key
	}
	if value := os.Getenv("SILICON_GITHUB_APP_ID"); value != "" {
		appID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || appID < 1 {
			return Config{}, errors.New("SILICON_GITHUB_APP_ID must be a positive integer")
		}
		cfg.GitHubAppID = appID
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
