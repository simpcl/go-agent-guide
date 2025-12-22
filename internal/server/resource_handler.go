package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go-agent-guide/internal/gateway"
	"go-x402-facilitator/pkg/types"

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
	// Register /resources routes
	resources := router.Group("/resources")
	{
		resources.GET("/discover", h.DiscoverResources)
	}

	api := router.Group("/api")
	{
		// Apply auth middleware first, then payment middleware
		api.Use(authMiddleware)
		api.Use(payMiddleware)
		// Catch-all route for api requests - must be last
		api.Any("/*path", h.HandleResourceRequest)
	}
}

// HandleResourceRequest handles requests to resources
// ResourceAuthMiddleware and ResourcePayMiddleware should have already validated and set resource_config in context
func (h *ResourceHandler) HandleResourceRequest(c *gin.Context) {
	// Reload resources if needed
	if err := h.resourceGateway.ReloadResourcesIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload resources")
	}

	// Get resource config from context (set by middleware if resource exists)
	resourceInterface, exists := c.Get("resource_config")
	if !exists {
		// Resource not found, return 404
		requestPath := c.Param("path")
		if !strings.HasPrefix(requestPath, "/") {
			requestPath = "/" + requestPath
		}
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
func (h *ResourceHandler) DiscoverResources(c *gin.Context) {
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
	response, err := h.discoverResources(c.Request.Context(), resourceType, limit, offset)
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

// discoverResources returns discovered resources from loaded configuration
func (h *ResourceHandler) discoverResources(ctx context.Context, resourceType string, limit, offset int) (*types.DiscoveryResponse, error) {
	return h.resourceGateway.DiscoverResources(ctx, resourceType, limit, offset)
}
