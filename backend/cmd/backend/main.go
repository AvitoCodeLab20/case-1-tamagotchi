package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/auth"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/config"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/database"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/httpserver"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/leaderboard"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/logging"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/rewards"
	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/storage"
)

func main() {
	logger := logging.New()

	if err := run(logger); err != nil {
		logger.Error("service stopped with error", "error", err)
		os.Exit(1)
	}

	logger.Info("service stopped")
}

func run(logger *logging.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	rootContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	databasePool, err := database.Open(rootContext, cfg.Database.URL, cfg.Database.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer databasePool.Close()

	authService, err := newAuthService(cfg.Auth, databasePool, logger)
	if err != nil {
		return fmt.Errorf("build auth service: %w", err)
	}

	leaderboardRepository := storage.NewLeaderboardRepository(databasePool)
	leaderboardService, err := leaderboard.NewService(leaderboardRepository)
	if err != nil {
		return fmt.Errorf("build leaderboard service: %w", err)
	}
	leaderboardFinalizer, err := leaderboard.NewFinalizer(leaderboardRepository)
	if err != nil {
		return fmt.Errorf("build leaderboard finalizer: %w", err)
	}
	rewardService, err := rewards.NewService(storage.NewRewardRepository(databasePool))
	if err != nil {
		return fmt.Errorf("build reward service: %w", err)
	}

	server, err := httpserver.New(httpserver.Options{
		Address:     cfg.HTTPAddress,
		Database:    databasePool,
		Auth:        authService,
		Leaderboard: leaderboardService,
		Rewards:     rewardService,
		Logger:      logger,
	})
	if err != nil {
		return fmt.Errorf("build HTTP server: %w", err)
	}

	serverErrors := make(chan error, 1)
	go runLeaderboardFinalizer(rootContext, logger, leaderboardFinalizer)

	go func() {
		logger.Info("http server started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-rootContext.Done():
		logger.Info("shutdown signal received")
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err = server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func newAuthService(cfg config.AuthConfig, pool *pgxpool.Pool, logger *logging.Logger) (*auth.Service, error) {
	tokenIssuer, err := auth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("build token issuer: %w", err)
	}

	passwordHasher, err := auth.NewPasswordHasher(cfg.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("build password hasher: %w", err)
	}

	service, err := auth.NewService(
		storage.NewUserRepository(pool),
		storage.NewRefreshSessionRepository(pool),
		tokenIssuer,
		passwordHasher,
		cfg.RefreshTokenTTL,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("build auth service: %w", err)
	}

	return service, nil
}

func runLeaderboardFinalizer(
	ctx context.Context,
	logger *logging.Logger,
	finalizer *leaderboard.Finalizer,
) {
	finalize := func() {
		if err := finalizer.FinalizePreviousWeek(ctx, time.Now()); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("leaderboard finalization failed", "error", err)
		}
	}
	finalize()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			finalize()
		}
	}
}
