package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	auditconst "github.com/RijalArul/disbursement-race-condition/internal/constants/audit"
	"github.com/RijalArul/disbursement-race-condition/internal/domain/models"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	auditsvc "github.com/RijalArul/disbursement-race-condition/internal/service/audit"
)

type AuditHandler struct {
	audits *auditsvc.Service
}

func NewAuditHandler(audits *auditsvc.Service) *AuditHandler {
	return &AuditHandler{audits: audits}
}

// List returns audit log entries with filtering, sorting, and pagination. Requires superadmin.
//
//	@Summary	List audit logs
//	@Tags		audit
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int		false	"Page number, default 1"
//	@Param		limit		query		int		false	"Page size, default 20, max 100"
//	@Param		entity_id	query		string	false	"Filter by the disbursement ID this entry is about"
//	@Param		action		query		string	false	"e.g. CREATED, APPROVED, REJECTED, DELETED"
//	@Param		date_from	query		string	false	"RFC3339 lower bound on created_at"
//	@Param		date_to		query		string	false	"RFC3339 upper bound on created_at"
//	@Param		sort_by		query		string	false	"Whitelisted column, e.g. created_at"
//	@Param		sort_order	query		string	false	"asc or desc"
//	@Success	200			{object}	response.Envelope{data=[]audit.Response,meta=response.PageMeta}
//	@Header		200			{string}	X-Request-ID	"Request correlation ID; echoed from the request header or generated if absent"
//	@Failure	400			{object}	response.Envelope	"VALIDATION_ERROR: sort_by/sort_order not in whitelist, or invalid date"
//	@Failure	401			{object}	response.Envelope	"UNAUTHORIZED"
//	@Failure	403			{object}	response.Envelope	"FORBIDDEN: caller is not superadmin"
//	@Router		/audit-logs [get]
func (h *AuditHandler) List(c *gin.Context) {
	req := auditconst.ListRequest{
		Page:      c.Query("page"),
		Limit:     c.Query("limit"),
		EntityID:  c.Query("entity_id"),
		Action:    c.Query("action"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}

	ctx := c.Request.Context()
	rows, meta, err := h.audits.List(ctx, req)
	if err != nil {
		response.MapError(c, err)
		return
	}

	response.OKWithMeta(c, http.StatusOK, toAuditResponseList(ctx, rows), meta)
}

func toAuditResponseList(ctx context.Context, rows []models.AuditLog) []auditconst.Response {
	out := make([]auditconst.Response, 0, len(rows))
	for i := range rows {
		out = append(out, toAuditResponse(ctx, &rows[i]))
	}
	return out
}

func toAuditResponse(ctx context.Context, a *models.AuditLog) auditconst.Response {
	before := unmarshalAuditField(ctx, a.ID, "before", a.Before)
	after := unmarshalAuditField(ctx, a.ID, "after", a.After)

	return auditconst.Response{
		ID:        a.ID,
		EntityID:  a.EntityID,
		Action:    a.Action,
		Actor:     a.ActorID,
		Before:    before,
		After:     after,
		CreatedAt: a.CreatedAt,
	}
}

func unmarshalAuditField(ctx context.Context, id, field string, raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		logger.FromCtx(ctx).Warn("audit log field failed to unmarshal", slog.String("audit_id", id), slog.String("field", field), slog.Any("error", err))
		return nil
	}
	return v
}
