package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/pagination"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/worker"
	"github.com/RijalArul/disbursement-race-condition/internal/repository/audit"
)

type Service struct {
	audits audit.AuditRepository
	pool   *worker.Pool
}

func NewService(audits audit.AuditRepository, pool *worker.Pool) *Service {
	return &Service{audits: audits, pool: pool}
}

// Event describes an audit-worthy change. Before is nil for action=created.
type Event struct {
	ActorID  string
	Action   string
	EntityID string
	Before   interface{}
	After    interface{}
}

// Enqueue submits an audit event to the worker pool. Must be called after the
// business transaction commits, never inside it — a failed audit insert must
// never fail the disbursement operation that produced it.
func (s *Service) Enqueue(ctx context.Context, ev Event) {
	// context.WithoutCancel: the worker runs after the handler returns, so the
	// request's cancellation must not abort the insert mid-flight. request_id
	// and the logger both travel with this detached ctx, since it carries the
	// same request-scoped values the request context had.
	detached := context.WithoutCancel(ctx)
	requestID := common.RequestIDFromCtx(ctx)
	log := logger.FromCtx(detached)

	s.pool.Enqueue(func() {
		if err := s.record(detached, ev, requestID); err != nil {
			log.Warn("audit insert failed", slog.String("action", ev.Action), slog.String("entity_id", ev.EntityID), slog.Any("error", err))
		}
	})
}

func (s *Service) record(ctx context.Context, ev Event, requestID string) error {
	before, err := json.Marshal(ev.Before)
	if err != nil {
		return fmt.Errorf("marshal audit before: %w", err)
	}
	after, err := json.Marshal(ev.After)
	if err != nil {
		return fmt.Errorf("marshal audit after: %w", err)
	}

	a := &models.AuditLog{
		ActorID:    ev.ActorID,
		Action:     ev.Action,
		EntityType: auditconst.EntityTypeDisbursement,
		EntityID:   ev.EntityID,
		Before:     before,
		After:      after,
		RequestID:  requestID,
	}

	if err := s.audits.Create(ctx, a); err != nil {
		return fmt.Errorf("create audit log for %s: %w", ev.EntityID, err)
	}
	return nil
}

// List validates query params, fetches the matching page, and computes pagination meta.
func (s *Service) List(ctx context.Context, in auditconst.ListRequest) ([]models.AuditLog, respconst.PageMeta, error) {
	q, err := validateList(in)
	if err != nil {
		return nil, respconst.PageMeta{}, err
	}

	rows, total, err := s.audits.List(ctx, q)
	if err != nil {
		return nil, respconst.PageMeta{}, fmt.Errorf("list audit logs: %w", err)
	}

	return rows, pagination.BuildPageMeta(q.Page, q.Limit, total), nil
}
