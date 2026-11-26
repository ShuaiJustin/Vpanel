// Package main is the entry point for the V Panel Node Agent.
// The Node Agent runs on each Xray node server and communicates with the Panel Server.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"v/internal/agent"
	"v/internal/logger"
)

var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/agent.yaml", "path to agent config file")
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("V Panel Node Agent %s\n", version)
		fmt.Printf("Build Time: %s\n", buildTime)
		fmt.Printf("Git Commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := agent.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	})

	log.Info("starting V Panel Node Agent",
		logger.F("version", version),
		logger.F("config", *configPath),
		logger.F("panel_url", cfg.Panel.URL),
	)

	// Create and start the agent
	nodeAgent, err := agent.New(cfg, log, version)
	if err != nil {
		log.Error("failed to create agent", logger.F("error", err))
		os.Exit(1)
	}

	// Start the agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := nodeAgent.Start(ctx); err != nil {
		log.Error("failed to start agent", logger.F("error", err))
		os.Exit(1)
	}

	log.Info("agent started",
		logger.F("node_name", cfg.Node.Name),
		logger.F("health_port", cfg.Health.Port),
	)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received", logger.F("signal", sig.String()))

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := nodeAgent.Stop(shutdownCtx); err != nil {
		log.Error("agent shutdown error", logger.F("error", err))
		os.Exit(1)
	}

	log.Info("agent stopped gracefully")
}
