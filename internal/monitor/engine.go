package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"domainmonitor/internal/classification"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/protocolcheck"
	"github.com/google/uuid"
)

type Engine struct {
	service   *Service
	protocols *protocolcheck.Suite
	workerID  string
	now       func() time.Time
}

func NewEngine(service *Service, protocols *protocolcheck.Suite, workerID string) *Engine {
	return &Engine{service: service, protocols: protocols, workerID: workerID, now: time.Now}
}

func (e *Engine) Execute(ctx context.Context, runID uuid.UUID) error {
	execution, err := e.service.ClaimExecution(ctx, runID, e.workerID)
	if err != nil {
		if errors.Is(err, ErrRunCompleted) || errors.Is(err, ErrRunExpired) {
			return nil
		}
		return err
	}
	runCtx, cancel := context.WithDeadline(ctx, execution.Run.DeadlineAt)
	defer cancel()

	localDNS := e.protocols.DNS.QueryAll(runCtx, e.protocols.LocalResolver, execution.Target.DomainASCII, 2)
	alternateDNS := e.protocols.DNS.QueryAll(runCtx, e.protocols.DoHResolver, execution.Target.DomainASCII, 2)
	mode := httpcheck.ContentMode(execution.Target.ExpectedContentMode)
	httpResult := e.protocols.HTTP.CheckDomain(runCtx, execution.Target.DomainASCII, mode)
	var pinnedHTTP *httpcheck.OriginResult
	if checker, ok := e.protocols.NewDoHPinnedHTTP(execution.Target.DomainASCII, alternateDNS); ok {
		result := checker.CheckDomain(runCtx, execution.Target.DomainASCII, mode)
		checker.CloseIdleConnections()
		pinnedHTTP = &result
	}
	decision := classification.Classify(localDNS, alternateDNS, httpResult)
	evidence := Evidence{
		LocalDNS: localDNS, AlternateDNS: alternateDNS, HTTP: httpResult, PinnedHTTP: pinnedHTTP,
		Decision: decision, CheckedAt: e.now().UTC(),
	}
	if err := e.service.CompleteRun(ctx, execution, evidence); err != nil {
		return fmt.Errorf("persist monitoring run %s: %w", runID, err)
	}
	return nil
}
