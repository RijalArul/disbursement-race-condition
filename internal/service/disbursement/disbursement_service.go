package disbursement

import (
	"context"
	"fmt"
	"log/slog"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
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
