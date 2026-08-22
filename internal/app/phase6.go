package app

import (
	"domainmonitor/internal/audit"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/recommendation"
	"domainmonitor/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Phase6Services struct {
	Recommendations *recommendation.Service
	Reports         *report.Service
}

func NewPhase6Services(pool *pgxpool.Pool, auditStore *audit.Store, financeService *finance.Service) Phase6Services {
	return Phase6Services{
		Recommendations: recommendation.NewService(pool, auditStore),
		Reports:         report.NewService(pool, auditStore, financeService),
	}
}
