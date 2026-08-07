package disbursement

import (
	"net/http"

	"github.com/gin-gonic/gin"

	disbconst "github.com/RijalArul/disbursement-race-condition/internal/constants/disbursement"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	disbsvc "github.com/RijalArul/disbursement-race-condition/internal/service/disbursement"
)

type Handler struct {
	disbursements *disbsvc.Service
}

func NewHandler(disbursements *disbsvc.Service) *Handler {
	return &Handler{disbursements: disbursements}
}

// Create submits a new disbursement for approval. Safe to retry with an Idempotency-Key.
//
//	@Summary	Create disbursement
//	@Tags		disbursements
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		Idempotency-Key		header		string								false	"UUID; a retried request with the same key and body within 24h replays the first response"
//	@Param		request				body		disbursement.CreateRequest			true	"Disbursement details"
//	@Success	201					{object}	response.Envelope{data=disbursement.Response}
//	@Header		201					{string}	X-Idempotent-Replayed	"true when this response is a replay of an earlier request with the same Idempotency-Key"
//	@Header		201					{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400					{object}	response.Envelope	"VALIDATION_ERROR: missing field or amount below minimum"
//	@Failure	401					{object}	response.Envelope	"UNAUTHORIZED: missing or invalid access token"
//	@Failure	409					{object}	response.Envelope	"IDEMPOTENCY_IN_PROGRESS: a request with this key is still processing"
//	@Failure	422					{object}	response.Envelope	"IDEMPOTENCY_KEY_REUSED: same key, different request body"
//	@Failure	429					{object}	response.Envelope	"RATE_LIMITED"
//	@Router		/disbursements [post]
func (h *Handler) Create(c *gin.Context) {
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

// GetByID returns a single disbursement by ID.
//
//	@Summary	Get disbursement
//	@Tags		disbursements
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Disbursement ID, e.g. DSB-000001"
//	@Success	200	{object}	response.Envelope{data=disbursement.Response}
//	@Header		200	{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	401	{object}	response.Envelope	"UNAUTHORIZED"
//	@Failure	404	{object}	response.Envelope	"NOT_FOUND: unknown ID or soft-deleted"
//	@Router		/disbursements/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	d, err := h.disbursements.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, toDisbursementResponse(d))
}

// UpdateStatus approves or rejects a PENDING disbursement. Requires admin or superadmin.
// approved_by is taken from the JWT, never the request body. Locked with SELECT ... FOR
// UPDATE so a concurrent approval and rejection on the same row cannot both succeed.
//
//	@Summary	Approve or reject a disbursement
//	@Tags		disbursements
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string								true	"Disbursement ID"
//	@Param		request	body		disbursement.UpdateStatusRequest		true	"New status: APPROVED or REJECTED"
//	@Success	200		{object}	response.Envelope{data=disbursement.Response}
//	@Header		200		{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400		{object}	response.Envelope	"VALIDATION_ERROR: status is neither APPROVED nor REJECTED"
//	@Failure	401		{object}	response.Envelope	"UNAUTHORIZED"
//	@Failure	403		{object}	response.Envelope	"FORBIDDEN: caller is not admin or superadmin"
//	@Failure	404		{object}	response.Envelope	"NOT_FOUND: unknown ID or soft-deleted"
//	@Failure	409		{object}	response.Envelope	"CONFLICT: disbursement is no longer PENDING"
//	@Router		/disbursements/{id}/status [patch]
func (h *Handler) UpdateStatus(c *gin.Context) {
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

// Delete soft-deletes a PENDING disbursement. Requires superadmin. Rows are never
// physically removed; a soft-deleted disbursement answers 404 on subsequent reads.
//
//	@Summary	Soft-delete a disbursement
//	@Tags		disbursements
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Disbursement ID"
//	@Success	204	"No content"
//	@Header		204	{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	401	{object}	response.Envelope	"UNAUTHORIZED"
//	@Failure	403	{object}	response.Envelope	"FORBIDDEN: caller is not superadmin"
//	@Failure	404	{object}	response.Envelope	"NOT_FOUND: unknown ID or already soft-deleted"
//	@Failure	409	{object}	response.Envelope	"CONFLICT: disbursement is no longer PENDING"
//	@Router		/disbursements/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.disbursements.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.MapError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// List returns disbursements with filtering, search, sorting, and pagination.
//
//	@Summary	List disbursements
//	@Tags		disbursements
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int		false	"Page number, default 1"
//	@Param		limit		query		int		false	"Page size, default 20, max 100"
//	@Param		search		query		string	false	"Matches recipient_name or account_number"
//	@Param		status		query		string	false	"PENDING, APPROVED, or REJECTED"
//	@Param		date_from	query		string	false	"RFC3339 lower bound on created_at"
//	@Param		date_to		query		string	false	"RFC3339 upper bound on created_at"
//	@Param		sort_by		query		string	false	"Whitelisted column, e.g. created_at, amount"
//	@Param		sort_order	query		string	false	"asc or desc"
//	@Success	200			{object}	response.Envelope{data=[]disbursement.Response,meta=response.PageMeta}
//	@Header		200			{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400			{object}	response.Envelope	"VALIDATION_ERROR: sort_by/sort_order not in whitelist, or invalid date"
//	@Failure	401			{object}	response.Envelope	"UNAUTHORIZED"
//	@Router		/disbursements [get]
func (h *Handler) List(c *gin.Context) {
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
