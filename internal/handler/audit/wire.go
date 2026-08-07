package audit

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	authmw "github.com/RijalArul/disbursement-race-condition/internal/middleware/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/ratelimit"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/worker"
	auditrepo "github.com/RijalArul/disbursement-race-condition/internal/repository/audit"
	auditsvc "github.com/RijalArul/disbursement-race-condition/internal/service/audit"
)

// New assembles the audit module's repository, service, and handler. It
// returns the service too since disbursement's wiring needs it as the
// AuditEnqueuer passed into disbsvc.NewService.
func New(db *gorm.DB, pool *worker.Pool) (*Handler, *auditsvc.Service) {
	repo := auditrepo.NewAuditRepository(db)
	svc := auditsvc.NewService(repo, pool)
	return NewHandler(svc), svc
}

// RegisterRoutes attaches GET /audit-logs, restricted to superadmin.
// Rate limit is per-user (JWT user_id), per BONUS-02.
func RegisterRoutes(r *gin.Engine, h *Handler, issuer *jwt.Issuer, defaultLimit int) {
	auditLogs := r.Group("/audit-logs", authmw.Auth(issuer), authmw.RequireRole(domain.RoleSuperAdmin), ratelimit.PerUser(defaultLimit, time.Minute))
	auditLogs.GET("", h.List)
}
