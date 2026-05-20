// Package main is the entry point for the V Panel application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	"v/internal/settings"
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

	// Apply startup-time overrides from the settings DB. Admin UI writes
	// fields like panel_port / log_level / timezone into the settings table;
	// without this step those fields are ignored. config.yaml still acts as
	// the fallback (only set fields override).
	applyStartupOverridesFromSettings(cfg, settings.NewService(repos.Settings), log)

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


// applyStartupOverridesFromSettings reads system settings persisted in the
// database (managed via the admin UI) and overrides matching fields on cfg.
// This is what makes admin UI changes to "panel port", "log retention", etc.
// actually take effect on the next restart. Only values the admin has
// explicitly written are applied — defaults still defer to config.yaml. We
// detect "explicitly set" by reading the raw key/value map and checking
// presence, which is how GetAll() differs from GetSystemSettings() (the
// latter overlays defaults and would clobber config.yaml on a fresh install).
func applyStartupOverridesFromSettings(cfg *config.Config, svc *settings.Service, log logger.Logger) {
	if cfg == nil || svc == nil {
		return
	}

	raw, err := svc.GetAll(context.Background())
	if err != nil {
		log.Warn("startup: failed to load settings overrides, using config.yaml only",
			logger.F("error", err),
		)
		return
	}

	// Intentionally NOT overriding Server.Host / Server.Port from settings DB.
	// Those are baked into container port mappings + reverse proxies and
	// must stay aligned with config.yaml / env vars. Admin UI displays them
	// for visibility but changing them requires editing config.yaml manually.
	// See the audit notes for context.

	if v, ok := raw["log_level"]; ok && strings.TrimSpace(v) != "" {
		cfg.Log.Level = strings.TrimSpace(v)
	}
	if v, ok := raw["log_retention_days"]; ok && strings.TrimSpace(v) != "" {
		var days int
		if _, perr := fmt.Sscanf(v, "%d", &days); perr == nil && days > 0 {
			cfg.Log.RetentionDays = days
		}
	}
	if v, ok := raw["log_path"]; ok {
		if p := strings.TrimSpace(v); p != "" {
			cfg.Log.Output = p
		}
	}

	if v, ok := raw["timezone"]; ok {
		tz := strings.TrimSpace(v)
		if tz != "" {
			if loc, locErr := time.LoadLocation(tz); locErr == nil {
				time.Local = loc
				_ = os.Setenv("TZ", tz)
			} else {
				log.Warn("startup: invalid timezone in settings, keeping previous",
					logger.F("timezone", tz),
					logger.F("error", locErr),
				)
			}
		}
	}

	// Panel base path (e.g. "/vpanel") — mounts the entire UI + API under
	// this prefix. Useful for reverse-proxy scenarios where the proxy does
	// NOT strip the prefix. Normalize so downstream consumers see either
	// "" or a leading-slash, no-trailing-slash form.
	if v, ok := raw["panel_base_path"]; ok {
		bp := strings.TrimSpace(v)
		bp = strings.TrimRight(bp, "/")
		if bp != "" && bp != "/" {
			if !strings.HasPrefix(bp, "/") {
				bp = "/" + bp
			}
			cfg.Server.BasePath = bp
		}
	}
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
