package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
)

func main() {
	// Parse command line flags
	var (
		configPath = flag.String("config", "config.yaml", "Path to configuration file")
		debug      = flag.Bool("debug", false, "Enable debug logging")
		version    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	// Show version and exit if requested
	if *version {
		fmt.Printf("PubSubGo Server v1.0.0\n")
		fmt.Printf("A high-performance message broker written in Go\n")
		os.Exit(0)
	}

	// Setup logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if *debug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	logger.WithFields(logrus.Fields{
		"config_path": *configPath,
		"debug":       *debug,
	}).Info("Starting PubSubGo server")

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Create application
	app, err := NewApplication(cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create application")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start application
	if err := app.Start(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to start application")
	}

	logger.Info("PubSubGo server started successfully")

	// Wait for shutdown signal
	sig := <-sigChan
	logger.WithField("signal", sig.String()).Info("Received shutdown signal")

	// Cancel context to signal shutdown
	cancel()

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.Stop(shutdownCtx); err != nil {
		logger.WithError(err).Error("Error during graceful shutdown")
		os.Exit(1)
	}

	logger.Info("PubSubGo server stopped gracefully")
}
