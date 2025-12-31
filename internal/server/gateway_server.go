package server

import (
	"context"
	"fmt"
	"net/http"

	"go-agent-guide/internal/config"
	"go-agent-guide/internal/gateway"
	"go-agent-guide/internal/middleware"
	"go-x402-facilitator/pkg/facilitator"

	"github.com/gin-gonic/gin"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
)

// GatewayServer represents the gateway HTTP server
// It handles resource requests with ResourceAuthMiddleware and ResourceX402SellerMiddleware
type GatewayServer struct {
	config          *config.Config
	facilitator     facilitator.PaymentFacilitator
	httpServer      *http.Server
	resourceGateway *gateway.ResourceGateway
	resourceHandler *ResourceHandler
}

// NewGatewayServer creates a new gateway HTTP server
func NewGatewayServer(cfg *config.Config, f facilitator.PaymentFacilitator) *GatewayServer {
	resourceGateway := gateway.NewResourceGateway(f, cfg)
	return &GatewayServer{
		config:          cfg,
		facilitator:     f,
		resourceGateway: resourceGateway,
		resourceHandler: NewResourceHandler(resourceGateway),
	}
}

// Start starts the gateway HTTP server
func (s *GatewayServer) Start() error {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.New()

	// Add basic middleware
	s.setupGatewayMiddleware(router)

	// Create resource-specific middlewares (auth and payment)
	authMiddleware := middleware.ResourceAuthMiddleware(s.resourceGateway)
	x402SellerMiddleware := middleware.ResourceX402SellerMiddleware(s.facilitator, s.resourceGateway)

	// Register resource routes
	s.resourceHandler.RegisterRoutes(router, authMiddleware, x402SellerMiddleware)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.GatewayServer.Host, s.config.GatewayServer.Port),
		Handler:      router,
		ReadTimeout:  s.config.GatewayServer.ReadTimeout,
		WriteTimeout: s.config.GatewayServer.WriteTimeout,
		IdleTimeout:  s.config.GatewayServer.IdleTimeout,
	}

	log.Info().
		Str("address", s.httpServer.Addr).
		Msg("Starting gateway HTTP server")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start gateway server: %w", err)
	}

	return nil
}

// Stop stops the gateway HTTP server gracefully
func (s *GatewayServer) Stop(ctx context.Context) error {
	log.Info().Msg("Shutting down gateway HTTP server")

	if s.httpServer == nil {
		return nil
	}

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown gateway server: %w", err)
	}

	log.Info().Msg("Gateway HTTP server stopped successfully")
	return nil
}

// setupGatewayMiddleware configures the middleware for the gateway server
func (s *GatewayServer) setupGatewayMiddleware(router *gin.Engine) {
	// Add logging middleware
	router.Use(gin.Logger())

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		Debug:          false,
	})
	router.Use(middleware.CorsMiddleware(c))

	// Add request ID middleware
	router.Use(middleware.RequestIDMiddleware())
}
