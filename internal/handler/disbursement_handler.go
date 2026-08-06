package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	disbsvc "github.com/RijalArul/disbursement-race-condition/internal/service/disbursement"
)

type DisbursementHandler struct {
	disbursements *disbsvc.Service
}

func NewDisbursementHandler(disbursements *disbsvc.Service) *DisbursementHandler {
	return &DisbursementHandler{disbursements: disbursements}
}

// Create handles POST /disbursements: bind, call service once, map. No business rule lives here.
func (h *DisbursementHandler) Create(c *gin.Context) {
	var req disbconst.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// A bind failure here is a malformed body or a type mismatch — for
		// example a string or a fractional number where amount expects int64.
		response.MapError(c, domain.Invalid(disbconst.InvalidBody))
		return
	}

	d, err := h.disbursements.Create(c.Request.Context(), disbconst.CreateInput{
		RecipientName: req.RecipientName,
		AccountNumber: req.AccountNumber,
		BankCode:      req.BankCode,
		Amount:        req.Amount,
		Note:          req.Note,
	})
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusCreated, toDisbursementResponse(d))
}

func toDisbursementResponse(d *models.Disbursement) disbconst.Response {
	return disbconst.Response{
		ID:            d.ID,
		RecipientName: d.RecipientName,
		AccountNumber: d.AccountNumber,
		BankCode:      d.BankCode,
		Note:          d.Note,
		Amount:        d.Amount,
		AdminFee:      d.AdminFee,
		Status:        string(d.Status),
		CreatedBy:     d.CreatedBy,
		ApprovedBy:    d.ApprovedBy,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}
