package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	codeInvalidActivityCode = "invalid_activity_code"
	codeActivityNotActive   = "activity_not_active"
	codeCooldownActive      = "cooldown_active"
	codeDailyLimitReached   = "daily_limit_reached"
	codeIdempotencyConflict = "idempotency_conflict"
)

const healthCheckTimeout = 2 * time.Second

type readinessChecker interface {
	Ping(context.Context) error
}

// Options carries everything the HTTP server needs. It is a struct rather than
// a parameter list so that adding a dependency does not touch every call site.
type Options struct {
	Address          string
	Database         readinessChecker
	Auth             authService
	Leaderboard      leaderboardService
	Rewards          rewardService
	Pet              petService
	Activity         activityService
	Progress         progressService
	Summary          dailySummaryService
	WebSocketOrigins []string
	Logger           *slog.Logger
}

// New builds the HTTP server with the routes mounted.
func New(options Options) (*http.Server, error) {
	switch {
	case options.Database == nil:
		return nil, errors.New("httpserver: database is required")
	case options.Auth == nil:
		return nil, errors.New("httpserver: auth service is required")
	case options.Pet == nil:
		return nil, errors.New("httpserver: pet service is required")
	case options.Activity == nil:
		return nil, errors.New("httpserver: activity service is required")
	case options.Summary == nil:
		return nil, errors.New("httpserver: daily summary service is required")
	case options.Leaderboard == nil:
		return nil, errors.New("httpserver: leaderboard service is required")
	case options.Rewards == nil:
		return nil, errors.New("httpserver: reward service is required")
	case options.Logger == nil:
		return nil, errors.New("httpserver: logger is required")
	case options.Progress == nil:
		return nil, errors.New("httpserver: progress service is required")
	}

	stateHub := newPetStateHub()
	ticketStore := newWebSocketTicketStore(webSocketTicketTTL)

	server := &http.Server{
		Addr:              options.Address,
		Handler:           newRouter(options, stateHub, ticketStore),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	server.RegisterOnShutdown(stateHub.Close)

	return server, nil
}

func newRouter(
	options Options,
	stateHub *petStateHub,
	ticketStore *webSocketTicketStore,
) http.Handler {
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
	mux.Handle("GET /api/v1/pet", chain(petHandler(options.Pet, options.Progress, options.Logger), authenticated))
	mux.Handle("GET /api/v1/activity-types", chain(activityTypesHandler(options.Activity, options.Logger), authenticated))
	mux.Handle("POST /api/v1/pet/actions", chain(performPetActionHandler(options.Activity, stateHub, options.Logger), authenticated))
	mux.Handle("GET /api/v1/progress", chain(progressHandler(options.Progress, options.Logger), authenticated))
	mux.Handle("GET /api/v1/daily-summaries/current", chain(currentDailySummaryHandler(options.Summary, options.Logger), authenticated))
	mux.Handle("GET /api/v1/daily-summaries/{summary_date}", chain(dailySummaryByDateHandler(options.Summary, options.Logger), authenticated))
	mux.Handle("POST /api/v1/ws-ticket", chain(
		webSocketTicketHandler(ticketStore, options.Logger),
		authenticated,
	))

	mux.Handle("GET /api/v1/ws/pet", petStateWebSocketHandler(
		options.Pet,
		ticketStore,
		stateHub,
		options.WebSocketOrigins,
		options.Logger,
	))
	mux.Handle("GET /api/v1/leaderboard/current", chain(
		currentLeaderboardHandler(options.Auth, options.Leaderboard, options.Logger),
		authenticated,
	))
	mux.Handle("GET /api/v1/rewards", chain(
		listRewardsHandler(options.Auth, options.Rewards, options.Logger),
		authenticated,
	))
	mux.Handle("POST /api/v1/rewards/{reward_id}/redeem", chain(
		redeemRewardHandler(options.Auth, options.Rewards, options.Logger),
		authenticated,
	))
	mux.Handle("POST /api/v1/leaderboard/awards/{award_id}/select", chain(
		selectLeaderboardAwardHandler(options.Auth, options.Rewards, options.Logger),
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
