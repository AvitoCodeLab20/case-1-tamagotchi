package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

type readinessChecker interface {
	Ping(context.Context) error
}

// Options carries everything the HTTP server needs. It is a struct rather than
// a parameter list so that adding a dependency does not touch every call site.
type Options struct {
	Address     string
	Database    readinessChecker
	Auth        authService
	Leaderboard leaderboardService
	Logger      *slog.Logger
}

// New builds the HTTP server with the routes mounted.
func New(options Options) (*http.Server, error) {
	switch {
	case options.Database == nil:
		return nil, errors.New("httpserver: database is required")
	case options.Auth == nil:
		return nil, errors.New("httpserver: auth service is required")
	case options.Leaderboard == nil:
		return nil, errors.New("httpserver: leaderboard service is required")
	case options.Logger == nil:
		return nil, errors.New("httpserver: logger is required")
	}

	return &http.Server{
		Addr:              options.Address,
		Handler:           newRouter(options),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func newRouter(options Options) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readinessHandler(options.Database, options.Logger))

	// The credential endpoints are throttled per client address: guessing a
	// password must not be cheaper than a handful of tries per minute.
	credentialLimit := throttle(newRateLimiter(defaultAuthRateBurst, defaultAuthRateWindow))
	authenticated := requireAuth(options.Auth)

	mux.Handle("POST /api/v1/auth/register", credentialLimit(registerHandler(options.Auth, options.Logger)))
	mux.Handle("POST /api/v1/auth/login", credentialLimit(loginHandler(options.Auth, options.Logger)))
	mux.Handle("POST /api/v1/auth/refresh", credentialLimit(refreshHandler(options.Auth, options.Logger)))
	mux.Handle("POST /api/v1/auth/logout", logoutHandler(options.Auth, options.Logger))
	mux.Handle("POST /api/v1/auth/logout-all", chain(logoutAllHandler(options.Auth, options.Logger), authenticated))
	mux.Handle("GET /api/v1/auth/me", chain(currentUserHandler(options.Auth, options.Logger), authenticated))
	mux.Handle("GET /api/v1/leaderboard/current", chain(
		currentLeaderboardHandler(options.Auth, options.Leaderboard, options.Logger),
		authenticated,
	))

	return mux
}

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func readinessHandler(database readinessChecker, logger *slog.Logger) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), healthCheckTimeout)
		defer cancel()

		if err := database.Ping(ctx); err != nil {
			logger.Warn("readiness check failed", "error", err)
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})

			return
		}

		writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
	}
}
