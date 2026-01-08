package middleware

import (
	"net/http"
	"strings"

	"go-agent-guide/internal/gateway"
	"github.com/agent-guide/go-x402-facilitator/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ResourceAuthMiddleware provides resource-specific authentication middleware
// It checks resources file to determine if authentication is required
func ResourceAuthMiddleware(resourceGateway *gateway.ResourceGateway) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Reload resources if needed
		if err := resourceGateway.ReloadResourcesIfNeeded(); err != nil {
			log.Warn().Err(err).Msg("Failed to reload resources")
		}

		// Get the requested path (use full URL path instead of param)
		requestPath := c.Request.URL.Path

		// Find resource configuration
		resource := resourceGateway.FindResource(requestPath)
		if resource == nil {
			// Resource not found, skip auth verification (will be handled by handler)
			c.Next()
			return
		}

		// Store resource in context for handler and other middlewares to use
		// (Set it even if auth is not required, so handler knows resource exists)
		c.Set("resource_config", resource)

		// Check if auth middleware is required for this resource
		hasAuth := false
		for _, mw := range resource.Middlewares {
			if mw == "auth" {
				hasAuth = true
				break
			}
		}

		if !hasAuth || resource.Auth == nil {
			// No auth requirement, continue
			c.Next()
			return
		}

		// Check authentication based on auth type
		if resource.Auth.Type == "bearer" {
			// Check Authorization header
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, types.ErrorResponse{
					Error:   "missing_authorization",
					Message: "Authorization header is required",
					Code:    http.StatusUnauthorized,
				})
				c.Abort()
				return
			}

			// Extract token from "Bearer <token>" format
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, types.ErrorResponse{
					Error:   "invalid_authorization_format",
					Message: "Authorization header must be in format 'Bearer <token>'",
					Code:    http.StatusUnauthorized,
				})
				c.Abort()
				return
			}

			token := parts[1]

			// Validate token matches resource configuration
			if token != resource.Auth.Token {
				c.JSON(http.StatusUnauthorized, types.ErrorResponse{
					Error:   "invalid_token",
					Message: "Invalid or expired token",
					Code:    http.StatusUnauthorized,
				})
				c.Abort()
				return
			}

			// Store token in context for potential use
			c.Set("auth_token", token)
		}

		// Authentication successful, continue to next handler
		c.Next()
	}
}
