package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/RijalArul/disbursement-race-condition/docs"
	"github.com/RijalArul/disbursement-race-condition/internal/config"
	audithdl "github.com/RijalArul/disbursement-race-condition/internal/handler/audit"
	authhdl "github.com/RijalArul/disbursement-race-condition/internal/handler/auth"
	disbhdl "github.com/RijalArul/disbursement-race-condition/internal/handler/disbursement"
	healthhdl "github.com/RijalArul/disbursement-race-condition/internal/handler/health"
	"github.com/RijalArul/disbursement-race-condition/internal/middleware/common"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/jwt"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/logger"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/response"
	"github.com/RijalArul/disbursement-race-condition/internal/pkg/worker"
)

// @title						Disbursement API
// @version					1.0
// @description				API disbursement dengan idempotency, approval yang aman terhadap race condition, soft delete, audit trail terpisah, dan structured logging dengan propagasi request ID.
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Ketik "Bearer" diikuti spasi dan access token, contoh: "Bearer eyJhbGciOi..."
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

	router := newRouter(cfg, db, sqlDB, auditPool)

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

func newRouter(cfg *config.Config, db *gorm.DB, sqlDB *sql.DB, auditPool *worker.Pool) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(common.RequestID(), common.Recovery(), common.AccessLog())

	r.NoRoute(func(c *gin.Context) {
		response.Err(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	healthhdl.RegisterRoutes(r, healthhdl.New(sqlDB))

	issuer := jwt.NewIssuer(cfg.JWTSecret, cfg.JWTAccessTTL)

	authHandler := authhdl.New(db, issuer, cfg.JWTRefreshTTL)
	authhdl.RegisterRoutes(r, authHandler, cfg.RateLimitLoginIP, cfg.RateLimitLoginUsername)

	auditHandler, auditService := audithdl.New(db, auditPool)
	audithdl.RegisterRoutes(r, auditHandler, issuer, cfg.RateLimitDefault)

	disbursementHandler := disbhdl.New(db, auditService)
	disbhdl.RegisterRoutes(r, disbursementHandler, db, issuer, cfg.RateLimitCreate, cfg.RateLimitDefault)

	return r
}
