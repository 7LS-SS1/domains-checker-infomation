package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/config"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/observability"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/probe"
	"domainmonitor/internal/protocolcheck"
	"domainmonitor/internal/queue"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration_invalid", "error", err)
		os.Exit(1)
	}
	logger := observability.NewLogger(cfg.Environment, "domain-monitor-worker")
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
	protocols, err := protocolcheck.New(cfg)
	if err != nil {
		logger.Error("protocol_suite_startup_failed", "error", err)
		os.Exit(1)
	}
	defer protocols.CloseIdleConnections()

	workerID := "worker-" + uuid.NewString()
	auditStore := audit.NewStore()
	service := monitor.NewService(pool, auditStore, cfg)
	engine := monitor.NewEngine(service, protocols, workerID)
	probeService := probe.NewService(pool, auditStore, cfg)
	consumer := queue.NewConsumer(redisClient, cfg.OutboxStream, cfg.MonitorQueueGroup, workerID,
		cfg.MonitorWorkers, cfg.MonitorQueueLease, cfg.MonitorQueueBlock,
		func(err error) { logger.Error("monitor_job_failed", "worker_id", workerID, "error", err) })
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info("worker_started", "worker_id", workerID, "concurrency", cfg.MonitorWorkers, "stream", cfg.OutboxStream)
	err = consumer.Run(ctx, func(ctx context.Context, values map[string]any) error {
		if eventType := fmt.Sprint(values["event_type"]); eventType != monitor.JobEventType {
			logger.WarnContext(ctx, "unsupported_queue_event_skipped", "worker_id", workerID, "event_type", eventType)
			return nil
		}
		job, err := decodeJob(values)
		if err != nil {
			return err
		}
		runID, err := uuid.Parse(job.RunID)
		if err != nil {
			return fmt.Errorf("invalid monitoring run ID: %w", err)
		}
		logger.InfoContext(ctx, "monitor_run_started", "worker_id", workerID, "monitor_run_id", runID)
		if err := engine.Execute(ctx, runID); err != nil {
			return err
		}
		logger.InfoContext(ctx, "monitor_run_completed", "worker_id", workerID, "monitor_run_id", runID)
		// Dispatch remote-probe (ISP) jobs right away instead of waiting for the
		// next scheduler tick, so cross-vantage ISP evidence refreshes as soon as
		// a local check completes — including forced ISP-check runs.
		if count, err := probeService.DispatchPending(ctx); err != nil {
			logger.ErrorContext(ctx, "remote_probe_dispatch_after_run_failed", "worker_id", workerID, "monitor_run_id", runID, "error", err)
		} else if count > 0 {
			logger.InfoContext(ctx, "remote_probe_jobs_dispatched_after_run", "worker_id", workerID, "monitor_run_id", runID, "count", count)
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		logger.Error("worker_stopped_with_error", "error", err)
		os.Exit(1)
	}
	logger.Info("worker_stopped")
}

func decodeJob(values map[string]any) (monitor.Job, error) {
	raw, ok := values["payload"]
	if !ok {
		return monitor.Job{}, fmt.Errorf("Redis job payload is missing")
	}
	var job monitor.Job
	if err := json.Unmarshal([]byte(fmt.Sprint(raw)), &job); err != nil {
		return monitor.Job{}, fmt.Errorf("decode monitoring job: %w", err)
	}
	if job.RunID == "" {
		return monitor.Job{}, fmt.Errorf("monitoring job run_id is missing")
	}
	return job, nil
}
