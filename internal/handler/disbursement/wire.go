package disbursement

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	authmw "github.com/RijalArul/disbursement-race-condition/internal/middleware/auth"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/idempotency"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	disbrepo "github.com/RijalArul/disbursement-race-condition/internal/repository/disbursement"
	idemrepo "github.com/RijalArul/disbursement-race-condition/internal/repository/idempotency"
	disbsvc "github.com/RijalArul/disbursement-race-condition/internal/service/disbursement"
)

// New assembles the disbursement module's repository, service, and handler.
// audits is the audit module's service, passed in as the AuditEnqueuer this
// service enqueues status-change events to.
func New(db *gorm.DB, audits disbsvc.AuditEnqueuer) *Handler {
	repo := disbrepo.NewDisbursementRepository(db)
	svc := disbsvc.NewService(repo, audits)
	return NewHandler(svc)
}

// RegisterRoutes attaches the /disbursements routes. The Idempotency-Key
// middleware and its repository are constructed here since POST /disbursements
// is their only consumer.
func RegisterRoutes(r *gin.Engine, h *Handler, db *gorm.DB, issuer *jwt.Issuer) {
	idempotencyRepo := idemrepo.NewIdempotencyRepository(db)

	// Auth is mandatory here, not decorative: the service reads created_by from
	// the identity this middleware puts on the request context.
	group := r.Group("/disbursements", authmw.Auth(issuer))
	group.POST("", idempotency.Middleware(idempotencyRepo), h.Create)
	group.GET("", h.List)
	group.GET("/:id", h.GetByID)
	group.PATCH("/:id/status", authmw.RequireRole(domain.RoleAdmin, domain.RoleSuperAdmin), h.UpdateStatus)
	group.DELETE("/:id", authmw.RequireRole(domain.RoleSuperAdmin), h.Delete)
}
