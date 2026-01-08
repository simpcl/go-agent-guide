package server

import (
	"context"
	"fmt"
	"go-agent-guide/internal/config"
	"go-agent-guide/internal/middleware"
	"github.com/agent-guide/go-x402-facilitator/pkg/facilitator"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
)

// AdminServer represents the admin HTTP server
// It handles management endpoints with AdminAuthMiddleware
type AdminServer struct {
	config      *config.Config
	facilitator facilitator.PaymentFacilitator
	httpServer  *http.Server
}

// NewAdminServer creates a new admin HTTP server
func NewAdminServer(cfg *config.Config, f facilitator.PaymentFacilitator) *AdminServer {
	return &AdminServer{
		config:      cfg,
		facilitator: f,
	}
}

// setupAdminMiddleware configures the middleware for the admin server
func (s *AdminServer) setupAdminMiddleware(router *gin.Engine) {
	// Add logging middleware
	router.Use(gin.Logger())

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		Debug:          s.config.AdminServer.LogLevel == "debug",
	})
	router.Use(middleware.CorsMiddleware(c))

	// Add authentication middleware if enabled
	if s.config.AdminServer.AuthEnabled {
		router.Use(middleware.AdminAuthMiddleware(s.config.AdminServer))
	}

	// Add metrics middleware if enabled
	if s.config.AdminServer.MetricsEnabled {
		router.Use(middleware.MetricsMiddleware())
	}

	// Add request ID middleware
	router.Use(middleware.RequestIDMiddleware())
}

// Start starts the admin HTTP server
func (s *AdminServer) Start() error {
	// Set Gin mode based on admin server log level
	if s.config.AdminServer.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	// Add middleware
	s.setupAdminMiddleware(router)

	// Register admin routes
	router.GET("/health", s.Health)
	router.GET("/ready", s.Ready)

	// Add metrics endpoint if enabled
	if s.config.AdminServer.MetricsEnabled {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.AdminServer.Host, s.config.AdminServer.Port),
		Handler:      router,
		ReadTimeout:  s.config.AdminServer.ReadTimeout,
		WriteTimeout: s.config.AdminServer.WriteTimeout,
		IdleTimeout:  s.config.AdminServer.IdleTimeout,
	}

	log.Info().
		Str("address", s.httpServer.Addr).
		Msg("Starting admin HTTP server")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start admin server: %w", err)
	}

	return nil
}

// Stop stops the admin HTTP server gracefully
func (s *AdminServer) Stop(ctx context.Context) error {
	log.Info().Msg("Shutting down admin HTTP server")

	if s.httpServer == nil {
		return nil
	}

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown admin server: %w", err)
	}

	log.Info().Msg("Admin HTTP server stopped successfully")
	return nil
}

// Health handles the /health endpoint
func (s *AdminServer) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// Ready handles the /ready endpoint
func (s *AdminServer) Ready(c *gin.Context) {
	if s.facilitator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"reason": "facilitator_not_initialized",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
