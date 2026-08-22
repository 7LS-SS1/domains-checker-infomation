package app

import (
	"log/slog"
	"net/http"

	"domainmonitor/internal/api"
	"domainmonitor/internal/audit"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/config"
	"domainmonitor/internal/domain"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/probe"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func NewAPIHandler(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, redisClient *redis.Client) http.Handler {
	auditStore := audit.NewStore()
	authService := auth.NewService(pool, auditStore, cfg.SessionTTL)
	domainStore := domain.NewStore(pool)
	domainService := domain.NewService(pool, domainStore, auditStore, domain.Normalizer{AllowUnknownTLD: cfg.AllowUnknownTLD})
	monitorService := monitor.NewService(pool, auditStore, cfg)
	probeService := probe.NewService(pool, auditStore, cfg)
	phase5 := NewPhase5Services(cfg, pool, auditStore)
	phase6 := NewPhase6Services(pool, auditStore, phase5.Finance)
	return api.NewServer(cfg, logger, pool, redisClient, authService, domainService, monitorService, probeService, api.IntelligenceServices{RDAP: phase5.RDAP, Finance: phase5.Finance, Sheets: phase5.Sheets, Drive: phase5.Drive, Recommendations: phase6.Recommendations, Reports: phase6.Reports}).Handler()
}
