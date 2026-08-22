package probe

import (
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool  *pgxpool.Pool
	audit *audit.Store
	cfg   config.Config
	now   func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, cfg config.Config) *Service {
	return &Service{pool: pool, audit: auditStore, cfg: cfg, now: time.Now}
}
