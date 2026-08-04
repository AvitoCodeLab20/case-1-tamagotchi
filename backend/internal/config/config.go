package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultHTTPAddress           = ":8080"
	defaultShutdownTimeout       = 10 * time.Second
	defaultDatabaseConnectTimout = 5 * time.Second
)

type Config struct {
	HTTPAddress            string
	ShutdownTimeout        time.Duration
	DatabaseURL            string
	DatabaseConnectTimeout time.Duration
}

func Load() (Config, error) {
	shutdownTimeout, err := durationFromEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	databaseConnectTimeout, err := durationFromEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimout)
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress:            envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		ShutdownTimeout:        shutdownTimeout,
		DatabaseURL:            databaseURL,
		DatabaseConnectTimeout: databaseConnectTimeout,
	}, nil
}

func databaseURLFromEnv() (string, error) {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		if _, err := url.ParseRequestURI(databaseURL); err != nil {
			return "", fmt.Errorf("parse DATABASE_URL: %w", err)
		}

		return databaseURL, nil
	}

	port := envOrDefault("POSTGRES_PORT", "5432")
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", fmt.Errorf("parse POSTGRES_PORT: %w", err)
	}

	databaseURL := &url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			envOrDefault("POSTGRES_USER", "postgres"),
			envOrDefault("POSTGRES_PASSWORD", "postgres"),
		),
		Host: net.JoinHostPort(envOrDefault("POSTGRES_HOST", "localhost"), port),
		Path: envOrDefault("POSTGRES_DB", "tamagotchi"),
	}

	query := databaseURL.Query()
	query.Set("sslmode", envOrDefault("POSTGRES_SSLMODE", "disable"))
	databaseURL.RawQuery = query.Encode()

	return databaseURL.String(), nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	rawValue := os.Getenv(name)
	if rawValue == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}

	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
