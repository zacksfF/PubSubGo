package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/zacksfF/PubSubGo/internal/config"
	"github.com/zacksfF/PubSubGo/internal/services/acknowledgment"
	"github.com/zacksfF/PubSubGo/internal/services/publisher"
	"github.com/zacksfF/PubSubGo/internal/services/subscriber"
	"github.com/zacksfF/PubSubGo/internal/services/topic"
)

// Server represents the HTTP server
type Server struct {
	server          *http.Server
	publisherSvc    publisher.Service
	subscriberSvc   subscriber.Service
	topicSvc        topic.Service
	acknowledgeSvc  acknowledgment.Service
	config          *config.BrokerConfig
	logger          *logrus.Logger
	wsHandler       func(w http.ResponseWriter, r *http.Request)
}

// NewServer creates a new HTTP server
func NewServer(
	publisherSvc publisher.Service,
	subscriberSvc subscriber.Service,
	topicSvc topic.Service,
	acknowledgeSvc acknowledgment.Service,
	config *config.BrokerConfig,
	logger *logrus.Logger,
) *Server {
	return &Server{
		publisherSvc:   publisherSvc,
		subscriberSvc:  subscriberSvc,
		topicSvc:       topicSvc,
		acknowledgeSvc: acknowledgeSvc,
		config:         config,
		logger:         logger,
	}
}

// SetWebSocketHandler sets the WebSocket handler
func (s *Server) SetWebSocketHandler(handler func(w http.ResponseWriter, r *http.Request)) {
	s.wsHandler = handler
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, port int) error {
	mux := s.setupRoutes()
	
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.logger.WithField("port", port).Info("Starting HTTP server")

	go func() {
		<-ctx.Done()
		s.logger.Info("Shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Stop stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// GetHandler returns the HTTP handler for the server
func (s *Server) GetHandler() http.Handler {
	return s.setupRoutes()
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health and metrics endpoints
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/metrics", s.metricsHandler)

	// WebSocket endpoint
	if s.wsHandler != nil {
		mux.HandleFunc("/ws", s.wsHandler)
	}

	// API v1 routes
	s.setupPublisherRoutes(mux)
	s.setupSubscriberRoutes(mux)
	s.setupTopicRoutes(mux)
	s.setupAcknowledgeRoutes(mux)

	// Apply middleware and return
	return s.withMiddleware(mux)
}

// withMiddleware applies middleware to the handler
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	// Chain middleware: logging -> CORS -> rate limiting -> next
	return s.withLogging(s.withCORS(s.withRateLimit(next)))
}