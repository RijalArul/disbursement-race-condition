package disbursement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/pagination"
	"github.com/RijalArul/disbursement-race-condition/internal/repository/disbursement"
	auditsvc "github.com/RijalArul/disbursement-race-condition/internal/service/audit"
)

// AuditEnqueuer is the audit-logging surface this service needs — just
// enough to submit an event, defined here (not in package audit) so tests
// fake it without a real worker pool or repository.
type AuditEnqueuer interface {
	Enqueue(ctx context.Context, ev auditsvc.Event)
}

type Service struct {
	disbursements disbursement.DisbursementRepository
	audits        AuditEnqueuer
}

func NewService(disbursements disbursement.DisbursementRepository, audits AuditEnqueuer) *Service {
	return &Service{disbursements: disbursements, audits: audits}
}

// Create validates input, derives server-controlled fields, and persists the disbursement.
func (s *Service) Create(ctx context.Context, in disbconst.CreateInput) (*models.Disbursement, error) {
	log := logger.FromCtx(ctx)

	identity, ok := common.IdentityFromCtx(ctx)
	if !ok || identity.UserID == "" {
		// Route should sit behind Auth middleware; refuse rather than write an unattributed row.
		log.Error("create disbursement reached service without an authenticated identity")
		return nil, domain.Unauthorized(disbconst.Unauthenticated)
	}

	input, err := validateCreate(in)
	if err != nil {
		return nil, err
	}

	d := &models.Disbursement{
		RecipientName: input.RecipientName,
		AccountNumber: input.AccountNumber,
		BankCode:      input.BankCode,
		Note:          input.Note,
		Amount:        input.Amount,
		AdminFee:      CalculateAdminFee(input.Amount),
		Status:        domain.StatusPending,
		CreatedBy:     identity.UserID,
		ApprovedBy:    nil,
	}

	if err := s.disbursements.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("create disbursement for user %s: %w", identity.UserID, err)
	}

	log.Info("disbursement created",
		slog.String("disbursement_id", d.ID),
		slog.Int64("amount", d.Amount),
		slog.Int64("admin_fee", d.AdminFee),
	)

	s.audits.Enqueue(ctx, auditsvc.Event{
		ActorID:  identity.UserID,
		Action:   auditconst.ActionCreated,
		EntityID: d.ID,
		Before:   nil,
		After:    d,
	})

	return d, nil
}

// List validates query params, fetches the matching page, and computes pagination meta.
func (s *Service) List(ctx context.Context, in disbconst.ListRequest) ([]models.Disbursement, respconst.PageMeta, error) {
	q, err := validateList(in)
	if err != nil {
		return nil, respconst.PageMeta{}, err
	}

	rows, total, err := s.disbursements.List(ctx, q)
	if err != nil {
		return nil, respconst.PageMeta{}, fmt.Errorf("list disbursements: %w", err)
	}

	return rows, pagination.BuildPageMeta(q.Page, q.Limit, total), nil
}

// GetByID returns a disbursement by id, open to any authenticated role.
func (s *Service) GetByID(ctx context.Context, id string) (*models.Disbursement, error) {
	d, err := s.disbursements.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NotFound(disbconst.NotFound)
		}
		return nil, fmt.Errorf("get disbursement %s: %w", id, err)
	}
	return d, nil
}

// mapRepoError maps ErrNotFound/ErrConflict to client-safe errors; ok is false for anything else.
func mapRepoError(err error, notFoundMsg, conflictMsg string) (mapped error, ok bool) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return domain.NotFound(notFoundMsg), true
	case errors.Is(err, domain.ErrConflict):
		return domain.Conflict(conflictMsg), true
	default:
		return nil, false
	}
}

// UpdateStatus approves or rejects a PENDING disbursement, racing on the row's FOR UPDATE lock so only one caller wins.
func (s *Service) UpdateStatus(ctx context.Context, in disbconst.UpdateStatusInput) (*models.Disbursement, error) {
	log := logger.FromCtx(ctx)

	if err := validateUpdateStatus(in.Status); err != nil {
		return nil, err
	}

	identity, _ := common.IdentityFromCtx(ctx)

	before, after, err := s.disbursements.UpdateStatus(ctx, in.ID, domain.DisbursementStatus(in.Status), identity.UserID, in.Note)
	if err != nil {
		conflictMsg := fmt.Sprintf(disbconst.AlreadyDecided, strings.ToLower(in.Status))
		if mapped, ok := mapRepoError(err, disbconst.NotFound, conflictMsg); ok {
			return nil, mapped
		}
		return nil, fmt.Errorf("update disbursement %s status: %w", in.ID, err)
	}

	log.Info("disbursement status updated",
		slog.String("disbursement_id", after.ID),
		slog.String("status", string(after.Status)),
		slog.String("approved_by", identity.UserID),
	)

	s.audits.Enqueue(ctx, auditsvc.Event{
		ActorID:  identity.UserID,
		Action:   auditconst.ActionStatusChanged,
		EntityID: after.ID,
		Before:   before,
		After:    after,
	})

	return after, nil
}

// Delete soft-deletes a PENDING disbursement, locked FOR UPDATE against a concurrent approve on the same row.
func (s *Service) Delete(ctx context.Context, id string) error {
	log := logger.FromCtx(ctx)

	identity, _ := common.IdentityFromCtx(ctx)

	before, err := s.disbursements.SoftDelete(ctx, id)
	if err != nil {
		if mapped, ok := mapRepoError(err, disbconst.NotFound, fmt.Sprintf(disbconst.NotPending, "deleted")); ok {
			return mapped
		}
		return fmt.Errorf("delete disbursement %s: %w", id, err)
	}

	log.Info("disbursement deleted", slog.String("disbursement_id", id))

	s.audits.Enqueue(ctx, auditsvc.Event{
		ActorID:  identity.UserID,
		Action:   auditconst.ActionDeleted,
		EntityID: id,
		Before:   before,
		After:    nil,
	})

	return nil
}
