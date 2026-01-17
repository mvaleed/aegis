package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/mvaleed/aegis/internal/auth"
	"github.com/mvaleed/aegis/internal/config"
	"github.com/mvaleed/aegis/internal/event"
	"github.com/mvaleed/aegis/internal/service"
	"github.com/mvaleed/aegis/internal/storage/postgres"
	grpcTransport "github.com/mvaleed/aegis/internal/transport/grpc"
	httpTransport "github.com/mvaleed/aegis/internal/transport/http"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup structured logging
	logLevel := slog.LevelInfo
	if cfg.Environment == "development" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Run the application
	if err := run(cfg, logger); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("connecting to database")
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	logger.Info("database connected")

	userRepo := postgres.NewUserRepository(pool)
	roleRepo := postgres.NewRoleRepository(pool)
	permissionRepo := postgres.NewPermissionRepository(pool)
	tokenRepo := postgres.NewTokenRepository(pool)

	jwtConfig := auth.JWTConfig{
		SecretKey:       cfg.JWTSecretKey,
		AccessTokenTTL:  cfg.AccessTokenTTL,
		RefreshTokenTTL: cfg.RefreshTokenTTL,
		Issuer:          "mvaleed",
		Audience:        []string{},
	}
	jwtManager := auth.NewJWTManager(
		jwtConfig,
	)

	// Initialize event publisher
	var publisher event.Publisher
	if cfg.IsDevelopment() {
		publisher = event.NewLoggingPublisher(logger)
	} else {
		// TODO: Real message broker
		publisher = event.NewLoggingPublisher(logger)
	}
	defer publisher.Close()

	userService := service.NewUserService(userRepo, roleRepo, publisher)
	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager, publisher)
	rbacService := service.NewRBACService(userRepo, roleRepo, permissionRepo, publisher)

	errChan := make(chan error, 2)

	httpServer := httpTransport.NewServer(
		cfg,
		userService,
		authService,
		rbacService,
		jwtManager,
		logger,
	)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.HTTPPort)
		logger.Info("starting HTTP server", "addr", addr)
		if err := httpServer.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	// Start gRPC server
	grpcServer := grpcTransport.NewServer(
		userService,
		authService,
		rbacService,
		jwtManager,
		logger,
	)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.GRPCPort)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			errChan <- fmt.Errorf("gRPC listen: %w", err)
			return
		}
		logger.Info("starting gRPC server", "addr", addr)
		if err := grpcServer.Serve(listener); err != nil && err != grpc.ErrServerStopped {
			errChan <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	// Token cleanup routine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := authService.CleanupExpiredTokens(ctx); err != nil {
					logger.Error("token cleanup failed", "error", err)
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("received shutdown signal", "signal", sig)
	case err := <-errChan:
		logger.Error("server error", "error", err)
		return err
	}

	logger.Info("initiating graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	grpcServer.GracefulStop()

	cancel()

	logger.Info("shutdown complete")
	return nil
}
