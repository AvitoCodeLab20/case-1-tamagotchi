package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

type readinessChecker interface {
	Ping(context.Context) error
}

func New(address string, database readinessChecker, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readinessHandler(database, logger))

	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
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

func writeJSON(response http.ResponseWriter, status int, payload map[string]string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)

	_ = json.NewEncoder(response).Encode(payload)
}
