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

// List handles GET /audit-logs: bind query params, call service once, map. No filtering/sorting logic lives here.
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
