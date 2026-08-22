package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"domainmonitor/internal/app"
	"domainmonitor/internal/buildinfo"
	"domainmonitor/internal/config"
	"domainmonitor/internal/observability"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration_invalid", "error", err)
		os.Exit(1)
	}
	logger := observability.NewLogger(cfg.Environment, "domain-monitor-api").With(
		"version", buildinfo.Version,
		"commit", buildinfo.Commit,
	)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer startupCancel()
	pool, err := postgres.Open(startupCtx, cfg)
	if err != nil {
		logger.Error("postgres_startup_failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	redisClient, err := rediscache.Open(startupCtx, cfg)
	if err != nil {
		logger.Error("redis_startup_failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = redisClient.Close() }()

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           app.NewAPIHandler(cfg, logger, pool, redisClient),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    32 << 10,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api_started", "address", cfg.HTTPAddress)
		serverErrors <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		logger.Info("shutdown_requested")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api_stopped_unexpectedly", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful_shutdown_failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api_stopped")
}
