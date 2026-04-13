package config

import (
	"log/slog"
	"os"
)

type Config struct {
	Port     string
	LogLevel slog.Level
}

func Load() Config {
	cfg := Config{
		Port:     "8080",
		LogLevel: slog.LevelInfo,
	}

	if p := os.Getenv("PORT"); p != "" {
		cfg.Port = p
	}

	if os.Getenv("LOG_LEVEL") == "debug" {
		cfg.LogLevel = slog.LevelDebug
	}

	return cfg
}
