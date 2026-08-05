package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
)

const (
	defaultHTTPAddress           = ":8080"
	defaultShutdownTimeout       = 10 * time.Second
	defaultDatabaseConnectTimout = 5 * time.Second

	defaultJWTIssuer       = "avito-tamagotchi"
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 30 * 24 * time.Hour
)

// Config is the full configuration of the backend service.
type Config struct {
	HTTPAddress     string
	ShutdownTimeout time.Duration
	Database        DatabaseConfig
	Auth            AuthConfig
}

// DatabaseConfig holds the PostgreSQL connection settings.
type DatabaseConfig struct {
	URL            string
	ConnectTimeout time.Duration
}

// AuthConfig holds the authentication settings. There is no default for the
// signing key on purpose: a service that starts with a built-in secret issues
// tokens anyone can forge.
type AuthConfig struct {
	JWTSecret       []byte
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	BcryptCost      int
}

// Load reads the configuration of the backend service.
func Load() (Config, error) {
	shutdownTimeout, err := durationFromEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}

	databaseConfig, err := LoadDatabase()
	if err != nil {
		return Config{}, err
	}

	authConfig, err := authFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress:     envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		ShutdownTimeout: shutdownTimeout,
		Database:        databaseConfig,
		Auth:            authConfig,
	}, nil
}

// LoadDatabase reads only the database settings.
//
// The migrate command uses it instead of Load: applying a schema change must
// not require the application secrets, so a migration can run in an environment
// that has no JWT_SECRET at all.
func LoadDatabase() (DatabaseConfig, error) {
	connectTimeout, err := durationFromEnv("DATABASE_CONNECT_TIMEOUT", defaultDatabaseConnectTimout)
	if err != nil {
		return DatabaseConfig{}, err
	}

	databaseURL, err := databaseURLFromEnv()
	if err != nil {
		return DatabaseConfig{}, err
	}

	return DatabaseConfig{URL: databaseURL, ConnectTimeout: connectTimeout}, nil
}

func authFromEnv() (AuthConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < auth.MinJWTSecretLength {
		return AuthConfig{}, fmt.Errorf(
			"JWT_SECRET is required and must be at least %d characters long", auth.MinJWTSecretLength)
	}

	accessTokenTTL, err := durationFromEnv("ACCESS_TOKEN_TTL", defaultAccessTokenTTL)
	if err != nil {
		return AuthConfig{}, err
	}

	refreshTokenTTL, err := durationFromEnv("REFRESH_TOKEN_TTL", defaultRefreshTokenTTL)
	if err != nil {
		return AuthConfig{}, err
	}
	if refreshTokenTTL <= accessTokenTTL {
		return AuthConfig{}, fmt.Errorf(
			"REFRESH_TOKEN_TTL (%s) must be longer than ACCESS_TOKEN_TTL (%s)", refreshTokenTTL, accessTokenTTL)
	}

	bcryptCost, err := bcryptCostFromEnv()
	if err != nil {
		return AuthConfig{}, err
	}

	return AuthConfig{
		JWTSecret:       []byte(secret),
		JWTIssuer:       envOrDefault("JWT_ISSUER", defaultJWTIssuer),
		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,
		BcryptCost:      bcryptCost,
	}, nil
}

func bcryptCostFromEnv() (int, error) {
	rawValue := os.Getenv("BCRYPT_COST")
	if rawValue == "" {
		return auth.DefaultBcryptCost, nil
	}

	cost, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, fmt.Errorf("parse BCRYPT_COST: %w", err)
	}
	// The upper bound keeps a typo from turning every login into a multi-second
	// CPU burn, which would be a denial of service against the service itself.
	if cost < auth.MinBcryptCost || cost > auth.MaxBcryptCost {
		return 0, fmt.Errorf("BCRYPT_COST must be between %d and %d", auth.MinBcryptCost, auth.MaxBcryptCost)
	}

	return cost, nil
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
