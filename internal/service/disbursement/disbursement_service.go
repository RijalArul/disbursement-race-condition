package disbursement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	respconst "github.com/RijalArul/disbursement-race-condition/internal/constants/response"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/repository"
)

type Service struct {
	disbursements repository.DisbursementRepository
}

func NewService(disbursements repository.DisbursementRepository) *Service {
	return &Service{disbursements: disbursements}
}

// Create validates input, derives server-controlled fields, and persists the disbursement.
func (s *Service) Create(ctx context.Context, in disbconst.CreateInput) (*models.Disbursement, error) {
	log := logger.FromCtx(ctx)

	identity, ok := middleware.IdentityFromCtx(ctx)
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

	// Audit event (action=created) lands with AUDIT-01, not here.

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

	return rows, buildPageMeta(q.Page, q.Limit, total), nil
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
