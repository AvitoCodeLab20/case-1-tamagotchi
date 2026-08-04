package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	clearConfigEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddress != defaultHTTPAddress {
		t.Errorf("HTTPAddress = %q, want %q", cfg.HTTPAddress, defaultHTTPAddress)
	}
	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Errorf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, defaultShutdownTimeout)
	}
	if !strings.Contains(cfg.DatabaseURL, "postgres://postgres:postgres@localhost:5432/tamagotchi") {
		t.Errorf("DatabaseURL = %q, want default PostgreSQL URL", cfg.DatabaseURL)
	}
}

func TestLoadDatabaseURLOverride(t *testing.T) {
	// Fixture DSN used only to check that DATABASE_URL takes precedence; not a real secret.
	const overrideDatabaseURL = "postgres://app:secret@database:5432/app?sslmode=require" //nolint:gosec // test fixture, not a real credential

	clearConfigEnvironment(t)
	t.Setenv("DATABASE_URL", overrideDatabaseURL)
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "3s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != overrideDatabaseURL {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.DatabaseConnectTimeout != 3*time.Second {
		t.Errorf("DatabaseConnectTimeout = %s, want 3s", cfg.DatabaseConnectTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("SHUTDOWN_TIMEOUT", "later")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"DATABASE_URL",
		"DATABASE_CONNECT_TIMEOUT",
		"HTTP_ADDR",
		"POSTGRES_DB",
		"POSTGRES_HOST",
		"POSTGRES_PASSWORD",
		"POSTGRES_PORT",
		"POSTGRES_SSLMODE",
		"POSTGRES_USER",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(name, "")
	}
}
