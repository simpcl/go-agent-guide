package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-agent-guide/internal/gateway"
	"github.com/agent-guide/go-x402-facilitator/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ResourceHandler handles HTTP routes for resources
type ResourceHandler struct {
	resourceGateway *gateway.ResourceGateway
}

// NewResourceHandler creates a new resource handler
func NewResourceHandler(resourceGateway *gateway.ResourceGateway) *ResourceHandler {
	return &ResourceHandler{
		resourceGateway: resourceGateway,
	}
}

// RegisterRoutes registers all API routes
func (h *ResourceHandler) RegisterRoutes(router *gin.Engine, authMiddleware, payMiddleware gin.HandlerFunc) {
	discover := router.Group("/discover")
	{
		discover.GET("/resources", h.HandleDiscoverResources)
	}

	// Reload resources if needed before registering routes
	if err := h.resourceGateway.ReloadResourcesIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload resources before registering routes")
	}

	// Get all resources and register a route for each
	resources := h.resourceGateway.GetAllResources()
	for _, resource := range resources {
		// Normalize resource path: remove trailing slash (except for root path "/")
		normalizedPath := resource.Resource
		if normalizedPath != "/" && strings.HasSuffix(normalizedPath, "/") {
			normalizedPath = strings.TrimSuffix(normalizedPath, "/")
		}

		// Create a route group for each resource
		resourceGroup := router.Group(normalizedPath)
		{
			// Apply auth middleware first, then payment middleware
			resourceGroup.Use(authMiddleware)
			resourceGroup.Use(payMiddleware)
			// Register both exact path and wildcard path to avoid 301 redirect
			// Exact path: matches /api/premium-data
			resourceGroup.Any("", h.HandleResourceRequest)
			// Wildcard path: matches /api/premium-data/*
			resourceGroup.Any("/*path", h.HandleResourceRequest)
		}
		log.Info().Str("resource", resource.Resource).Str("normalized", normalizedPath).Msg("Registered route for resource")
	}
}

// HandleResourceRequest handles requests to resources
func (h *ResourceHandler) HandleResourceRequest(c *gin.Context) {
	// Reload resources if needed
	if err := h.resourceGateway.ReloadResourcesIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload resources")
	}

	// Get resource config from context (set by middleware if resource exists)
	resourceInterface, exists := c.Get("resource_config")
	if !exists {
		// Resource not found, return 404
		requestPath := c.Request.URL.Path
		c.JSON(http.StatusNotFound, types.ErrorResponse{
			Error:   "resource_not_found",
			Message: fmt.Sprintf("Resource not found: %s", requestPath),
			Code:    http.StatusNotFound,
		})
		return
	}

	resource, ok := resourceInterface.(*gateway.ResourceConfig)
	if !ok || resource == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "internal_error",
			Message: "Invalid resource config in context",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// All middlewares passed, proxy the request
	h.resourceGateway.ProxyRequest(c, resource)
}

// DiscoverResources handles the /resources/discover endpoint
func (h *ResourceHandler) HandleDiscoverResources(c *gin.Context) {
	// Parse query parameters
	resourceType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Call facilitator
	response, err := h.resourceGateway.DiscoverResources(c.Request.Context(), resourceType, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Facilitator discover resources failed")
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "internal_error",
			Message: "Internal server error during resource discovery",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
