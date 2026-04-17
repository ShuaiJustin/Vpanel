// Package main is the entry point for the V Panel application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"v/internal/auth"
	"v/internal/config"
	"v/internal/database"
	"v/internal/database/repository"
	logservice "v/internal/log"
	"v/internal/logger"
	"v/internal/proxy"
	"v/internal/proxy/protocols/shadowsocks"
	"v/internal/proxy/protocols/trojan"
	"v/internal/proxy/protocols/vless"
	"v/internal/proxy/protocols/vmess"
	"v/internal/server"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("V Panel %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg.Version = version

	// Initialize logger
	log := logger.New(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	})

	log.Info("starting V Panel",
		logger.F("version", version),
		logger.F("config", *configPath),
	)

	// Initialize database
	db, err := database.New(&database.Config{
		Driver: cfg.Database.Driver,
		DSN:    cfg.Database.DSN,
	})
	if err != nil {
		log.Error("failed to initialize database", logger.F("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		log.Error("failed to run migrations", logger.F("error", err))
		os.Exit(1)
	}

	// Initialize repositories
	repos := repository.NewRepositories(db.DB())

	// Initialize log service
	logService := logservice.NewService(repos.Log, log, logservice.Config{
		DatabaseEnabled: cfg.Log.DatabaseEnabled,
		DatabaseLevel:   cfg.Log.DatabaseLevel,
		RetentionDays:   cfg.Log.RetentionDays,
		BufferSize:      cfg.Log.BufferSize,
		BatchSize:       cfg.Log.BatchSize,
		FlushInterval:   cfg.Log.FlushInterval,
	})

	// Start cleanup scheduler
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	logService.StartCleanupScheduler(cleanupCtx)

	log.Info("log service initialized",
		logger.F("database_enabled", cfg.Log.DatabaseEnabled),
		logger.F("database_level", cfg.Log.DatabaseLevel),
		logger.F("retention_days", cfg.Log.RetentionDays),
	)

	// Initialize auth service
	authService := auth.NewService(auth.Config{
		JWTSecret:           cfg.Auth.JWTSecret,
		TokenExpiry:         cfg.Auth.TokenExpiry,
		RefreshTokenExpiry:  cfg.Auth.RefreshTokenExpiry,
	})

	// Ensure system roles exist
	if err := repos.Role.EnsureSystemRoles(context.Background()); err != nil {
		log.Error("failed to ensure system roles", logger.F("error", err))
		os.Exit(1)
	}

	// Ensure default admin user exists
	if err := ensureAdminUser(repos.User, authService, cfg, log); err != nil {
		log.Error("failed to ensure admin user", logger.F("error", err))
		os.Exit(1)
	}

	// Initialize proxy manager
	proxyManager := proxy.NewManager(repos.Proxy)

	// Register protocols
	proxyManager.RegisterProtocol(vmess.New())
	proxyManager.RegisterProtocol(vless.New())
	proxyManager.RegisterProtocol(trojan.New())
	proxyManager.RegisterProtocol(shadowsocks.New())

	// Create and start server
	srv := server.New(cfg, log, authService, proxyManager, repos, logService)

	if err := srv.Start(); err != nil {
		log.Error("failed to start server", logger.F("error", err))
		os.Exit(1)
	}

	log.Info("server started",
		logger.F("address", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
	)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", logger.F("signal", sig.String()))

	// Stop cleanup scheduler
	cleanupCancel()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Error("server shutdown error", logger.F("error", err))
		os.Exit(1)
	}

	// Close log service (flushes remaining logs)
	if err := logService.Close(); err != nil {
		log.Error("log service shutdown error", logger.F("error", err))
	}

	log.Info("server stopped gracefully")
}


// ensureAdminUser creates the default admin user if it doesn't exist.
// If the user exists, it updates the password to match the configuration.
func ensureAdminUser(userRepo repository.UserRepository, authService *auth.Service, cfg *config.Config, log logger.Logger) error {
	ctx := context.Background()

	// Hash the password from config
	passwordHash, err := authService.HashPassword(cfg.Auth.AdminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	// Check if admin user exists
	existingUser, err := userRepo.GetByUsername(ctx, cfg.Auth.AdminUsername)
	if err == nil {
		// Admin user already exists
		updated := false
		
		// Update password if different
		if existingUser.PasswordHash != passwordHash {
			existingUser.PasswordHash = passwordHash
			updated = true
		}
		
		// Set default display name if empty
		if existingUser.DisplayName == "" {
			existingUser.DisplayName = "系统管理员"
			updated = true
		}
		
		if updated {
			if err := userRepo.Update(ctx, existingUser); err != nil {
				return fmt.Errorf("failed to update admin user: %w", err)
			}
			log.Info("admin user updated", logger.F("username", cfg.Auth.AdminUsername))
		} else {
			log.Info("admin user already exists", logger.F("username", cfg.Auth.AdminUsername))
		}
		return nil
	}

	// Create admin user
	adminUser := &repository.User{
		Username:     cfg.Auth.AdminUsername,
		PasswordHash: passwordHash,
		Email:        "",
		DisplayName:  "系统管理员", // Set default display name
		Role:         "admin",
		Enabled:      true,
	}

	if err := userRepo.Create(ctx, adminUser); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Info("admin user created", logger.F("username", cfg.Auth.AdminUsername))
	return nil
}
