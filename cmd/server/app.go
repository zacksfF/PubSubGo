package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	httpserver "github.com/zacksfF/PubSubGo/internal/adapters/api/http"
	"github.com/zacksfF/PubSubGo/internal/adapters/api/websocket"
	prometheusadapter "github.com/zacksfF/PubSubGo/internal/adapters/metrics/prometheus"
	redisadapter "github.com/zacksfF/PubSubGo/internal/adapters/storage/redis"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/ports/repository"
	"github.com/zacksfF/PubSubGo/internal/services/acknowledgment"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
	"github.com/zacksfF/PubSubGo/internal/services/topic"
	"github.com/zacksfF/PubSubGo/pkg/tracing"
)

// Application represents the main server application
type Application struct {
	config *config.Config
	logger *logrus.Logger

	// Storage
	redisClient *goredis.Client

	// Repositories
	messageRepo      repository.MessageRepository
	topicRepo        repository.TopicRepository
	subscriptionRepo repository.SubscriptionRepository
	consumerRepo     repository.ConsumerGroupRepository

	// Services
	publisherSvc   publisher.Service
	subscriberSvc  subscriber.Service
	topicSvc       topic.Service
	acknowledgeSvc acknowledgment.Service

	// API Servers
	httpServer    *http.Server
	wsServer      *websocket.Server
	metricsServer *http.Server

	// Metrics
	metricsCollector *prometheusadapter.MetricsCollector

	// Tracing
	tracer *tracing.Tracer

	// Shutdown coordination
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config, logger *logrus.Logger) (*Application, error) {
	app := &Application{
		config: cfg,
		logger: logger,
	}

	// Initialize tracing
	if err := app.initTracing(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize storage
	if err := app.initStorage(); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize services
	if err := app.initServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	// Initialize API servers
	if err := app.initServers(); err != nil {
		return nil, fmt.Errorf("failed to initialize servers: %w", err)
	}

	return app, nil
}

// initStorage initializes storage components
func (a *Application) initStorage() error {
	// Create Redis client
	addr := "localhost:6379"
	if len(a.config.Redis.Addresses) > 0 {
		addr = a.config.Redis.Addresses[0]
	}

	a.redisClient = goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     a.config.Redis.Password,
		DB:           a.config.Redis.DB,
		PoolSize:     a.config.Redis.PoolSize,
		MinIdleConns: a.config.Redis.MinIdleConns,
		MaxRetries:   a.config.Redis.MaxRetries,
		ReadTimeout:  a.config.Redis.ReadTimeout,
		WriteTimeout: a.config.Redis.WriteTimeout,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	a.logger.Info("Connected to Redis successfully")

	// Initialize Redis adapter client
	redisClient, err := redisadapter.NewClient(&a.config.Redis)
	if err != nil {
		return fmt.Errorf("failed to create Redis adapter client: %w", err)
	}

	// Initialize repositories
	a.messageRepo = redisadapter.NewMessageRepository(redisClient)
	a.topicRepo = redisadapter.NewTopicRepository(redisClient)
	a.subscriptionRepo = redisadapter.NewSubscriptionRepository(redisClient)
	a.consumerRepo = redisadapter.NewConsumerGroupRepository(redisClient)

	return nil
}

// initTracing initializes the tracing system
func (a *Application) initTracing() error {
	tracerConfig := tracing.TracerConfig{
		Enabled:      a.config.Tracing.Enabled,
		ServiceName:  a.config.Tracing.ServiceName,
		OTLPEndpoint: a.config.Tracing.OTLPEndpoint,
		SamplingRate: a.config.Tracing.SamplingRate,
	}

	tracer, err := tracing.New(tracerConfig)
	if err != nil {
		return fmt.Errorf("failed to create tracer: %w", err)
	}

	a.tracer = tracer
	a.logger.WithFields(logrus.Fields{
		"enabled":      tracerConfig.Enabled,
		"service_name": tracerConfig.ServiceName,
		"endpoint":     tracerConfig.OTLPEndpoint,
	}).Info("Tracing initialized")

	return nil
}

// initServices initializes business logic services
func (a *Application) initServices() error {
	// Initialize metrics
	a.metricsCollector = prometheusadapter.NewMetricsCollector()
	// Metrics are registered within the NewMetricsCollector

	// Initialize services
	a.publisherSvc = publisher.NewService(
		a.messageRepo,
		a.topicRepo,
		&a.config.Broker,
		a.logger,
	)

	a.subscriberSvc = subscriber.NewService(
		a.messageRepo,
		a.subscriptionRepo,
		a.consumerRepo,
		a.topicRepo,
		&a.config.Broker,
		a.logger,
	)

	a.topicSvc = topic.NewService(
		a.topicRepo,
		a.messageRepo,
		&a.config.Broker,
		a.logger,
	)

	a.acknowledgeSvc = acknowledgment.NewService(
		a.messageRepo,
		&a.config.Broker,
		a.logger,
	)

	a.logger.Info("Services initialized successfully")
	return nil
}

// initServers initializes HTTP and WebSocket servers
func (a *Application) initServers() error {
	// Create WebSocket server
	a.wsServer = websocket.NewServer(
		a.publisherSvc,
		a.subscriberSvc,
		a.topicSvc,
		&a.config.Broker,
		a.logger,
	)

	// Create HTTP API server
	httpAPIServer := httpserver.NewServer(
		a.publisherSvc,
		a.subscriberSvc,
		a.topicSvc,
		a.acknowledgeSvc,
		&a.config.Broker,
		a.logger,
	)

	// Set WebSocket handler
	httpAPIServer.SetWebSocketHandler(a.wsServer.HandleWebSocket)

	a.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", a.config.Server.Port),
		Handler:      httpAPIServer.GetHandler(),
		ReadTimeout:  a.config.Server.ReadTimeout,
		WriteTimeout: a.config.Server.WriteTimeout,
		IdleTimeout:  a.config.Server.IdleTimeout,
	}

	// Create metrics server
	a.metricsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.config.Metrics.Port),
		Handler: promhttp.Handler(),
	}

	a.logger.Info("Servers initialized successfully")
	return nil
}

// Start starts all application components
func (a *Application) Start(ctx context.Context) error {
	ctx, a.cancel = context.WithCancel(ctx)

	// Start acknowledgment service workers
	if err := a.acknowledgeSvc.StartAcknowledgmentWorker(ctx); err != nil {
		return fmt.Errorf("failed to start acknowledgment workers: %w", err)
	}

	// Start HTTP server
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.WithField("port", a.config.Server.Port).Info("Starting HTTP server")

		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.WithError(err).Error("HTTP server error")
		}
	}()

	// WebSocket is handled separately or integrated into the HTTP routes

	// Start metrics server
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.WithField("port", a.config.Metrics.Port).Info("Starting metrics server")

		if err := a.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.WithError(err).Error("Metrics server error")
		}
	}()

	// Start background workers
	a.startBackgroundWorkers(ctx)

	return nil
}

// Stop gracefully stops all application components
func (a *Application) Stop(ctx context.Context) error {
	a.logger.Info("Shutting down application")

	var shutdownErrors []error

	// Cancel context to stop background workers
	if a.cancel != nil {
		a.cancel()
	}

	// Stop HTTP server
	if a.httpServer != nil {
		if err := a.httpServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("HTTP server shutdown error: %w", err))
		}
	}

	// Stop metrics server
	if a.metricsServer != nil {
		if err := a.metricsServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("metrics server shutdown error: %w", err))
		}
	}

	// Stop WebSocket server
	if a.wsServer != nil {
		if err := a.wsServer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("WebSocket server shutdown error: %w", err))
		}
	}

	// Stop acknowledgment service workers
	if a.acknowledgeSvc != nil {
		if err := a.acknowledgeSvc.StopAcknowledgmentWorker(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("acknowledgment workers shutdown error: %w", err))
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("All goroutines stopped")
	case <-ctx.Done():
		a.logger.Warn("Shutdown timeout exceeded")
	}

	// Close Redis connection
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("Redis close error: %w", err))
		}
	}

	// Shutdown tracer
	if a.tracer != nil {
		if err := a.tracer.Shutdown(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("tracer shutdown error: %w", err))
		}
	}

	// Return any shutdown errors
	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	return nil
}

// startBackgroundWorkers starts background workers for message processing
func (a *Application) startBackgroundWorkers(ctx context.Context) {
	// Start subscription workers
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.Info("Starting subscription workers")

		// This would start background workers for processing subscriptions
		// For now, we'll just wait for context cancellation
		<-ctx.Done()
		a.logger.Info("Subscription workers stopped")
	}()

	// Start acknowledgment workers
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.Info("Starting acknowledgment workers")

		// This would start background workers for processing acknowledgments
		// For now, we'll just wait for context cancellation
		<-ctx.Done()
		a.logger.Info("Acknowledgment workers stopped")
	}()

	// Start metrics collection worker
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.logger.Info("Starting metrics collection worker")

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				a.logger.Info("Metrics collection worker stopped")
				return
			case <-ticker.C:
				// Collect periodic metrics
				a.collectMetrics()
			}
		}
	}()
}

// collectMetrics collects and updates periodic metrics
func (a *Application) collectMetrics() {
	// Update connection metrics for WebSocket server
	if a.wsServer != nil {
		stats := a.wsServer.GetConnectionStats()
		a.logger.WithFields(logrus.Fields{
			"total_connections":   stats["total_connections"],
			"alive_connections":   stats["alive_connections"],
			"total_subscriptions": stats["total_subscriptions"],
		}).Debug("WebSocket connection stats")
	}

	// Additional metrics collection can be added here
}
