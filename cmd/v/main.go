// Package main is the entry point for the V Panel application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"v/internal/auth"
	"v/internal/backup"
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

	// Daily DB snapshot scheduler. Gated by VPANEL_BACKUP_ENABLED so ops
	// can opt out if they're running their own backup tooling (Litestream,
	// volume snapshots, etc.).
	if os.Getenv("VPANEL_BACKUP_ENABLED") != "0" {
		retention := 14
		if v := strings.TrimSpace(os.Getenv("VPANEL_BACKUP_RETENTION_DAYS")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				retention = n
			}
		}
		dbPath := os.Getenv("V_DB_PATH")
		if dbPath == "" {
			dbPath = cfg.Database.DSN
		}
		backupSvc := backup.New(dbPath, retention, 3, log)
		backupSvc.Start(cleanupCtx)
	}

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

	// Override runtime config from settings DB. Admin can change panel_port,
	// panel_access_ip, log_*, timezone, panel_base_path from the UI; we read
	// them here at startup so the next "重启面板" picks up the new values.
	// config.yaml / env vars remain the fallback (only set fields override).
	//
	// Caveat for port/host changes under Docker: the container port mapping
	// in docker-compose.yml is external to the panel. Admin who changes
	// panel_port via UI is expected to also update the compose port mapping;
	// the UI warns about this explicitly.
	if v, ok := raw["panel_access_ip"]; ok {
		if ip := strings.TrimSpace(v); ip != "" {
			cfg.Server.Host = ip
		}
	}
	if v, ok := raw["panel_port"]; ok {
		if p := strings.TrimSpace(v); p != "" {
			var port int
			if _, perr := fmt.Sscanf(p, "%d", &port); perr == nil && port > 0 && port < 65536 {
				cfg.Server.Port = port
			}
		}
	}

	// TLS overrides — admin can paste paths via UI. Both must be set for
	// the server to switch to HTTPS at next restart; setting one of them
	// (mismatched) leaves the server on HTTP to avoid a half-broken state.
	certPath := strings.TrimSpace(raw["panel_cert_path"])
	keyPath := strings.TrimSpace(raw["panel_key_path"])
	if certPath != "" && keyPath != "" {
		cfg.Server.TLSCert = certPath
		cfg.Server.TLSKey = keyPath

		// When TLS is enabled by the admin via UI ("应用并保存"), automatically
		// upgrade PublicURL and CORS allowlist entries from http:// to https://
		// so that generated links (subscription URLs, password reset emails,
		// invite links) and CORS checks match the actual served scheme. This
		// removes the foot-gun of admin enabling HTTPS in UI but still serving
		// http:// links because .env was never touched.
		//
		// Reverse-proxy deployments (proxy terminates TLS, panel speaks HTTP
		// internally) are not affected: in those, panel_cert_path stays empty.
		if upgraded := upgradeURLToHTTPS(cfg.Server.PublicURL); upgraded != cfg.Server.PublicURL {
			log.Info("startup: PublicURL upgraded to https due to TLS being enabled",
				logger.F("before", cfg.Server.PublicURL),
				logger.F("after", upgraded))
			cfg.Server.PublicURL = upgraded
		}
		for i, o := range cfg.Server.CORSOrigins {
			cfg.Server.CORSOrigins[i] = upgradeURLToHTTPS(o)
		}
	}

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

// upgradeURLToHTTPS rewrites a URL's scheme from http:// to https:// while
// keeping the rest of the URL intact. Empty / non-http inputs are returned
// unchanged. Used by applyStartupOverridesFromSettings so admin doesn't
// have to keep .env in sync with the UI "应用证书" action.
func upgradeURLToHTTPS(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return u
	}
	if strings.HasPrefix(u, "http://") {
		return "https://" + strings.TrimPrefix(u, "http://")
	}
	return u
}
