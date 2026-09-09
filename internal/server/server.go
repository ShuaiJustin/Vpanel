// Package server provides HTTP server with graceful shutdown.
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"v/internal/api"
	"v/internal/auth"
	"v/internal/certificate"
	"v/internal/config"
	"v/internal/database/repository"
	logservice "v/internal/log"
	"v/internal/logger"
	"v/internal/proxy"
)

// Server represents the HTTP server.
type Server struct {
	config             *config.Config
	logger             logger.Logger
	httpServer         *http.Server
	router             *api.Router
	authService        *auth.Service
	proxyManager       proxy.Manager
	repos              *repository.Repositories
	logService         *logservice.Service
	certificateService *certificate.Service
	listener           net.Listener
	runCancel          context.CancelFunc
}

// New creates a new Server.
func New(
	cfg *config.Config,
	log logger.Logger,
	authService *auth.Service,
	proxyManager proxy.Manager,
	repos *repository.Repositories,
	logService *logservice.Service,
) *Server {
	// 初始化证书服务
	certificateService := certificate.NewService(
		repos.Certificate,
		repos.Node,
		repos.CertificateDeployment,
		log,
		cfg.Certificate.StoragePath,
	).WithAutoRenewConfig(cfg.Certificate.CheckInterval, cfg.Certificate.RenewThreshold)
	certificateService.SetProxyRepository(repos.Proxy)

	return &Server{
		config:             cfg,
		logger:             log,
		authService:        authService,
		proxyManager:       proxyManager,
		repos:              repos,
		logService:         logService,
		certificateService: certificateService,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	// Create router
	s.router = api.NewRouter(
		s.config,
		s.logger,
		s.authService,
		s.proxyManager,
		s.repos,
		s.logService,
		s.certificateService,
	)

	// Bind before starting any business schedulers. A duplicate process or an
	// invalid listen address must fail startup without running background jobs.
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	listener, err := prepareListener(s.config, addr)
	if err != nil {
		return err
	}
	s.listener = listener
	if err := os.MkdirAll(s.config.Certificate.StoragePath, 0o750); err != nil {
		_ = listener.Close()
		s.listener = nil
		return fmt.Errorf("create certificate storage directory: %w", err)
	}
	outboxDir, err := notificationOutboxDir(s.config)
	if err != nil {
		_ = listener.Close()
		s.listener = nil
		return fmt.Errorf("resolve notification outbox directory: %w", err)
	}
	if err := s.router.ConfigureNotificationOutbox(outboxDir); err != nil {
		_ = listener.Close()
		s.listener = nil
		return fmt.Errorf("configure notification outbox: %w", err)
	}
	s.router.Setup()
	ctx, cancel := context.WithCancel(context.Background())
	s.runCancel = cancel
	s.router.StartNotificationServices(ctx)
	s.router.StartCommercialScheduler()

	// Start health checker
	if err := s.router.StartHealthChecker(ctx); err != nil {
		s.logger.Warn("健康检查服务启动失败，继续启动服务器", logger.Err(err))
		// 不阻止服务器启动
	}
	if err := s.router.StartNodeTrafficResetScheduler(ctx); err != nil {
		s.logger.Warn("节点流量重置调度器启动失败，继续启动服务器", logger.Err(err))
	}
	if err := s.router.StartRuntimeReconciler(ctx); err != nil {
		s.logger.Warn("运行时巡检器启动失败，继续启动服务器", logger.Err(err))
	}

	// Certificate expiry/deployment monitoring always runs. The setting only
	// controls whether ACME renewals are attempted.
	if err := s.certificateService.StartMonitoring(ctx, s.config.Certificate.AutoRenewEnabled); err != nil {
		s.logger.Warn("证书监控服务启动失败，继续启动服务器", logger.Err(err))
	} else {
		s.logger.Info("证书监控服务已启动",
			logger.F("auto_renew", s.config.Certificate.AutoRenewEnabled),
			logger.F("check_interval", s.config.Certificate.CheckInterval),
			logger.F("renew_threshold", s.config.Certificate.RenewThreshold))
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router.Engine(),
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		s.logger.Info("starting HTTP server",
			logger.F("address", addr),
			logger.F("mode", s.config.Server.Mode),
		)

		var err error
		if s.config.Server.TLSCert != "" && s.config.Server.TLSKey != "" {
			err = s.httpServer.ServeTLS(listener, s.config.Server.TLSCert, s.config.Server.TLSKey)
		} else {
			err = s.httpServer.Serve(listener)
		}

		if err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", logger.F("error", err))
			cancel()
		}
	}()

	return nil
}

func prepareListener(cfg *config.Config, addr string) (net.Listener, error) {
	if cfg.Server.TLSCert != "" || cfg.Server.TLSKey != "" {
		if cfg.Server.TLSCert == "" || cfg.Server.TLSKey == "" {
			return nil, fmt.Errorf("both TLS certificate and key are required")
		}
		if _, err := tls.LoadX509KeyPair(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil {
			return nil, fmt.Errorf("load TLS certificate: %w", err)
		}
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return listener, nil
}

func notificationOutboxDir(cfg *config.Config) (string, error) {
	dataDir := strings.TrimSpace(os.Getenv("VPANEL_DATA_DIR"))
	if dataDir == "" {
		dbPath := strings.TrimSpace(cfg.Database.Path)
		if dbPath != "" {
			dataDir = filepath.Dir(dbPath)
		} else {
			storagePath := strings.TrimSpace(cfg.Certificate.StoragePath)
			if storagePath != "" {
				dataDir = filepath.Dir(storagePath)
			} else {
				dataDir = "data"
			}
		}
	}
	absDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(absDir, "notification-outbox"), nil
}

// Stop stops the HTTP server gracefully.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")
	if s.runCancel != nil {
		s.runCancel()
	}

	// 停止证书自动续期服务
	if s.certificateService != nil {
		if err := s.certificateService.StopAutoRenew(); err != nil {
			s.logger.Warn("证书自动续期服务停止失败", logger.Err(err))
		}
	}

	// Stop health checker and schedulers
	if s.router != nil {
		s.router.StopCommercialScheduler()
		if err := s.router.StopRuntimeReconciler(ctx); err != nil {
			s.logger.Warn("运行时巡检器停止失败", logger.Err(err))
		}
		if err := s.router.StopNodeTrafficResetScheduler(ctx); err != nil {
			s.logger.Warn("节点流量重置调度器停止失败", logger.Err(err))
		}
		if err := s.router.StopHealthChecker(ctx); err != nil {
			s.logger.Warn("健康检查服务停止失败", logger.Err(err))
		}
	}

	if s.httpServer == nil {
		return nil
	}

	// Shutdown with context timeout
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", logger.F("error", err))
		return err
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// Run starts the server and waits for shutdown signal.
func (s *Server) Run() error {
	// Start server
	if err := s.Start(); err != nil {
		return err
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	s.logger.Info("shutdown signal received")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Server.ShutdownTimeout)
	defer cancel()

	// Stop server
	return s.Stop(ctx)
}

// GracefulShutdown performs graceful shutdown with the given timeout.
func (s *Server) GracefulShutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Stop(ctx)
}

// Address returns the server address.
func (s *Server) Address() string {
	if s.httpServer == nil {
		return ""
	}
	return s.httpServer.Addr
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	return s.httpServer != nil
}
