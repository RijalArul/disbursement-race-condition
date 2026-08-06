package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/RijalArul/disbursement-race-condition/internal/config"
	"github.com/RijalArul/disbursement-race-condition/internal/domain"
	"github.com/RijalArul/disbursement-race-condition/internal/handler"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/idempotency"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/worker"
	"github.com/RijalArul/disbursement-race-condition/internal/repository"
	auditsvc "github.com/RijalArul/disbursement-race-condition/internal/service/audit"
	authsvc "github.com/RijalArul/disbursement-race-condition/internal/service/auth"
	disbsvc "github.com/RijalArul/disbursement-race-condition/internal/service/disbursement"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	logger.Init(cfg.LogLevel)
	log := logger.FromCtx(context.Background())

	db, err := connectDB(cfg)
	if err != nil {
		log.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("failed to get underlying sql.DB", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sqlDB.Close()

	auditPool := worker.NewPool(cfg.AuditWorkerCount, cfg.AuditBufferSize, log)

	router := newRouter(cfg, db, auditPool)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	go func() {
		log.Info("server starting", slog.String("port", cfg.AppPort), slog.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received, draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	auditPool.Shutdown()

	log.Info("server exited cleanly")
}

func connectDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	return db, nil
}

func newRouter(cfg *config.Config, db *gorm.DB, auditPool *worker.Pool) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recovery(), middleware.AccessLog())

	r.NoRoute(func(c *gin.Context) {
		response.Err(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})

	r.GET("/health", func(c *gin.Context) {
		response.OK(c, http.StatusOK, gin.H{"status": "ok"})
	})

	issuer := jwt.NewIssuer(cfg.JWTSecret, cfg.JWTAccessTTL)
	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	authService := authsvc.NewService(userRepo, refreshTokenRepo, issuer, cfg.JWTRefreshTTL)
	authHandler := handler.NewAuthHandler(authService)

	auth := r.Group("/auth")
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/logout", authHandler.Logout)

	auditRepo := repository.NewAuditRepository(db)
	auditService := auditsvc.NewService(auditRepo, auditPool)
	auditHandler := handler.NewAuditHandler(auditService)

	disbursementRepo := repository.NewDisbursementRepository(db)
	disbursementService := disbsvc.NewService(disbursementRepo, auditService)
	disbursementHandler := handler.NewDisbursementHandler(disbursementService)
	idempotencyRepo := repository.NewIdempotencyRepository(db)

	// Auth is mandatory here, not decorative: the service reads created_by from
	// the identity this middleware puts on the request context.
	disbursements := r.Group("/disbursements", middleware.Auth(issuer))
	disbursements.POST("", idempotency.Middleware(idempotencyRepo), disbursementHandler.Create)
	disbursements.GET("", disbursementHandler.List)
	disbursements.GET("/:id", disbursementHandler.GetByID)
	disbursements.PATCH("/:id/status", middleware.RequireRole(domain.RoleAdmin, domain.RoleSuperAdmin), disbursementHandler.UpdateStatus)
	disbursements.DELETE("/:id", middleware.RequireRole(domain.RoleSuperAdmin), disbursementHandler.Delete)

	auditLogs := r.Group("/audit-logs", middleware.Auth(issuer), middleware.RequireRole(domain.RoleSuperAdmin))
	auditLogs.GET("", auditHandler.List)

	return r
}
