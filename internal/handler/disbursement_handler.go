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

// GetByID handles GET /disbursements/:id.
func (h *DisbursementHandler) GetByID(c *gin.Context) {
	d, err := h.disbursements.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, toDisbursementResponse(d))
}

// UpdateStatus handles PATCH /disbursements/:id/status: bind, call service once, map.
func (h *DisbursementHandler) UpdateStatus(c *gin.Context) {
	var req disbconst.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.MapError(c, domain.Invalid(disbconst.InvalidBody))
		return
	}

	d, err := h.disbursements.UpdateStatus(c.Request.Context(), disbconst.UpdateStatusInput{
		ID:     c.Param("id"),
		Status: req.Status,
		Note:   req.Note,
	})
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, toDisbursementResponse(d))
}

// Delete handles DELETE /disbursements/:id: bind, call service once, map.
func (h *DisbursementHandler) Delete(c *gin.Context) {
	if err := h.disbursements.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.MapError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// List handles GET /disbursements: bind query params, call service once, map. No filtering/sorting logic lives here.
func (h *DisbursementHandler) List(c *gin.Context) {
	req := disbconst.ListRequest{
		Page:      c.Query("page"),
		Limit:     c.Query("limit"),
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	rows, meta, err := h.disbursements.List(c.Request.Context(), req)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OKWithMeta(c, http.StatusOK, toDisbursementResponseList(rows), meta)
}

func toDisbursementResponseList(rows []models.Disbursement) []disbconst.Response {
	out := make([]disbconst.Response, 0, len(rows))
	for i := range rows {
		out = append(out, toDisbursementResponse(&rows[i]))
	}
	return out
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
