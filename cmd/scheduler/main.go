package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"domainmonitor/internal/app"
	"domainmonitor/internal/audit"
	"domainmonitor/internal/config"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/observability"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/probe"
	"domainmonitor/internal/queue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration_invalid", "error", err)
		os.Exit(1)
	}
	logger := observability.NewLogger(cfg.Environment, "domain-monitor-scheduler")
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

	dispatcher := queue.NewDispatcher(pool, redisClient, cfg.OutboxBatchSize, cfg.OutboxLease, cfg.OutboxMaxAttempts)
	auditStore := audit.NewStore()
	monitorService := monitor.NewService(pool, auditStore, cfg)
	probeService := probe.NewService(pool, auditStore, cfg)
	phase5 := app.NewPhase5Services(cfg, pool, auditStore)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(cfg.SchedulerInterval)
	defer ticker.Stop()
	logger.Info("scheduler_started", "capabilities", []string{"due_schedule_claim", "outbox_dispatch", "remote_probe_dispatch", "google_sheet_preview"})
	for {
		if count, err := monitorService.ScheduleDue(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("scheduler_stopped")
				return
			}
			logger.Error("monitor_schedule_failed", "error", err)
		} else if count > 0 {
			logger.Info("monitor_runs_scheduled", "count", count)
		}
		if count, err := dispatcher.RunOnce(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("scheduler_stopped")
				return
			}
			logger.Error("outbox_dispatch_failed", "error", err)
		} else if count > 0 {
			logger.Info("outbox_dispatched", "count", count)
		}
		if count, err := probeService.DispatchPending(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("scheduler_stopped")
				return
			}
			logger.Error("remote_probe_dispatch_failed", "error", err)
		} else if count > 0 {
			logger.Info("remote_probe_jobs_dispatched", "count", count, "region", cfg.ProbeRequiredRegion)
		}
		if count, err := phase5.Sheets.ScheduleDue(ctx); err != nil {
			if ctx.Err() != nil {
				logger.Info("scheduler_stopped")
				return
			}
			logger.Error("google_sheet_schedule_failed", "error", err)
		} else if count > 0 {
			logger.Info("google_sheet_preview_created", "count", count)
		}
		select {
		case <-ctx.Done():
			logger.Info("scheduler_stopped")
			return
		case <-ticker.C:
		}
	}
}
