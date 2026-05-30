package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/leyl1ne/ApiGateway/internal/auth/jwt"
	config "github.com/leyl1ne/ApiGateway/internal/config/gateway"
	"github.com/leyl1ne/ApiGateway/internal/http/router"
	"github.com/leyl1ne/ApiGateway/internal/logger"
	"github.com/leyl1ne/ApiGateway/internal/logger/zl"
)

func main() {

	// ─── Конфигурация ───────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// ─── Логгер ─────────────────────────────────────────────────────────
	zeroLog, err := zl.NewZerologLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
		os.Exit(1)
	}

	zeroLog.Info("starting api-gateway")

	zeroLog.Info("configuration loaded",
		logger.Field{Key: "port", Value: cfg.Server.HTTP.Port},
		logger.Field{Key: "user_service_url", Value: cfg.Services.UserService.URL},
	)

	// ─── JWT Validator ──────────────────────────────────────────────────
	jwtValidator := jwt.NewJWTValidator(jwt.Config{
		Secret:         cfg.JWT.Secret,
		AccessTokenTTL: cfg.JWT.AccessTokenTTL,
	})

	// ─── Роутер ─────────────────────────────────────────────────────────
	engine := router.Setup(cfg, jwtValidator, zeroLog)

	// ─── HTTP-сервер ────────────────────────────────────────────────────
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
		Handler:      engine,
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout,
		WriteTimeout: cfg.Server.HTTP.WriteTimeout,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("api-gateway failed", logger.Err(err))
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.GracefulShutdown)
	defer shutdownCancel()

	if err := s.Shutdown(shutdownCtx); err != nil {
		zeroLog.Error("failed to stop api-gateway gracefully", logger.Err(err))

		if err := s.Close(); err != nil {
			zeroLog.Error("forced shutdown failed", logger.Err(err))
		}
		return
	}

	zeroLog.Info("api-gateway exists")
}
