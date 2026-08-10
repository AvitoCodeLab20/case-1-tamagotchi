package config

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
)

// testJWTSecret is a fixture signing key, long enough to pass the length check.
const testJWTSecret = "test-signing-key-that-is-long-enough"

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
	if !strings.Contains(cfg.Database.URL, "postgres://postgres:postgres@localhost:5432/tamagotchi") {
		t.Errorf("Database.URL = %q, want default PostgreSQL URL", cfg.Database.URL)
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

	if cfg.Database.URL != overrideDatabaseURL {
		t.Errorf("Database.URL = %q", cfg.Database.URL)
	}
	if cfg.Database.ConnectTimeout != 3*time.Second {
		t.Errorf("Database.ConnectTimeout = %s, want 3s", cfg.Database.ConnectTimeout)
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

func TestLoadAuthDefaults(t *testing.T) {
	clearConfigEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if string(cfg.Auth.JWTSecret) != testJWTSecret {
		t.Errorf("JWTSecret = %q, want the configured secret", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.JWTIssuer != defaultJWTIssuer {
		t.Errorf("JWTIssuer = %q, want %q", cfg.Auth.JWTIssuer, defaultJWTIssuer)
	}
	if cfg.Auth.AccessTokenTTL != defaultAccessTokenTTL {
		t.Errorf("AccessTokenTTL = %s, want %s", cfg.Auth.AccessTokenTTL, defaultAccessTokenTTL)
	}
	if cfg.Auth.RefreshTokenTTL != defaultRefreshTokenTTL {
		t.Errorf("RefreshTokenTTL = %s, want %s", cfg.Auth.RefreshTokenTTL, defaultRefreshTokenTTL)
	}
	if cfg.Auth.BcryptCost != auth.DefaultBcryptCost {
		t.Errorf("BcryptCost = %d, want %d", cfg.Auth.BcryptCost, auth.DefaultBcryptCost)
	}
}

// TestLoadDatabaseWorksWithoutTheJWTSecret guards the split between the two
// loaders: the migrate command must keep running in environments that have no
// application secrets, such as the migrations job in CI.
func TestLoadDatabaseWorksWithoutTheJWTSecret(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("JWT_SECRET", "")

	cfg, err := LoadDatabase()
	if err != nil {
		t.Fatalf("LoadDatabase() error = %v", err)
	}
	if !strings.Contains(cfg.URL, "postgres://") {
		t.Errorf("URL = %q, want a PostgreSQL DSN", cfg.URL)
	}
	if cfg.ConnectTimeout != defaultDatabaseConnectTimout {
		t.Errorf("ConnectTimeout = %s, want %s", cfg.ConnectTimeout, defaultDatabaseConnectTimout)
	}

	// The full loader, in the same environment, must still refuse to start.
	if _, err = Load(); err == nil {
		t.Error("Load() error = nil, want a missing secret error")
	}
}

// TestLoadRequiresJWTSecret is the reason there is no default: a service that
// starts with a built-in signing key issues tokens anyone can forge.
func TestLoadRequiresJWTSecret(t *testing.T) {
	testCases := []struct {
		name   string
		secret string
	}{
		{name: "missing", secret: ""},
		{name: "too short", secret: strings.Repeat("x", auth.MinJWTSecretLength-1)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clearConfigEnvironment(t)
			t.Setenv("JWT_SECRET", testCase.secret)

			if _, err := Load(); err == nil {
				t.Error("Load() error = nil, want a missing secret error")
			}
		})
	}
}

func TestLoadRejectsInvalidAuthSettings(t *testing.T) {
	testCases := []struct {
		name  string
		name2 string
		env   map[string]string
	}{
		{
			name: "refresh TTL not longer than access TTL",
			env:  map[string]string{"ACCESS_TOKEN_TTL": "1h", "REFRESH_TOKEN_TTL": "30m"},
		},
		{
			name: "bcrypt cost below the minimum",
			env:  map[string]string{"BCRYPT_COST": strconv.Itoa(auth.MinBcryptCost - 1)},
		},
		{
			name: "bcrypt cost above the maximum",
			env:  map[string]string{"BCRYPT_COST": strconv.Itoa(auth.MaxBcryptCost + 1)},
		},
		{
			name: "bcrypt cost is not a number",
			env:  map[string]string{"BCRYPT_COST": "strong"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clearConfigEnvironment(t)

			for name, value := range testCase.env {
				t.Setenv(name, value)
			}

			if _, err := Load(); err == nil {
				t.Error("Load() error = nil, want a validation error")
			}
		})
	}
}

func TestLoadAuthOverrides(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("JWT_ISSUER", "avito-tamagotchi-staging")
	t.Setenv("ACCESS_TOKEN_TTL", "5m")
	t.Setenv("REFRESH_TOKEN_TTL", "168h")
	t.Setenv("BCRYPT_COST", strconv.Itoa(auth.MinBcryptCost))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Auth.JWTIssuer != "avito-tamagotchi-staging" {
		t.Errorf("JWTIssuer = %q", cfg.Auth.JWTIssuer)
	}
	if cfg.Auth.AccessTokenTTL != 5*time.Minute {
		t.Errorf("AccessTokenTTL = %s, want 5m", cfg.Auth.AccessTokenTTL)
	}
	if cfg.Auth.RefreshTokenTTL != 168*time.Hour {
		t.Errorf("RefreshTokenTTL = %s, want 168h", cfg.Auth.RefreshTokenTTL)
	}
	if cfg.Auth.BcryptCost != auth.MinBcryptCost {
		t.Errorf("BcryptCost = %d, want %d", cfg.Auth.BcryptCost, auth.MinBcryptCost)
	}
}

// clearConfigEnvironment resets every variable Load reads and sets the one
// mandatory secret, so each test starts from documented defaults.
func clearConfigEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"ACCESS_TOKEN_TTL",
		"BCRYPT_COST",
		"DATABASE_URL",
		"DATABASE_CONNECT_TIMEOUT",
		"HTTP_ADDR",
		"JWT_ISSUER",
		"POSTGRES_DB",
		"POSTGRES_HOST",
		"POSTGRES_PASSWORD",
		"POSTGRES_PORT",
		"POSTGRES_SSLMODE",
		"POSTGRES_USER",
		"REFRESH_TOKEN_TTL",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(name, "")
	}

	t.Setenv("JWT_SECRET", testJWTSecret)
}
