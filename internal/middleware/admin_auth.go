package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"go-agent-guide/internal/config"
	"go-x402-facilitator/pkg/types"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware provides authentication middleware for admin server
// Supports bearer, basic, and api_key authentication types
func AdminAuthMiddleware(authConfig config.AdminServerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for health endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/ready" {
			c.Next()
			return
		}

		switch authConfig.AuthType {
		case "bearer":
			validateBearerAuth(c, authConfig.AuthTokens)
		case "basic":
			validateBasicAuth(c, authConfig.AuthTokens)
		case "api_key":
			validateAPIKeyAuth(c, authConfig.AuthTokens)
		default:
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Error:   "invalid_auth_config",
				Message: "Invalid authentication type configured",
				Code:    http.StatusInternalServerError,
			})
			c.Abort()
			return
		}

		if !c.IsAborted() {
			c.Next()
		}
	}
}

// validateBearerAuth validates Bearer token authentication
func validateBearerAuth(c *gin.Context, validTokens []string) {
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
	if !isValidToken(token, validTokens) {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "invalid_token",
			Message: "Invalid or expired token",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	c.Set("auth_token", token)
}

// validateBasicAuth validates Basic authentication
// For basic auth, tokens should be in format "username:password" (base64 encoded)
func validateBasicAuth(c *gin.Context, validTokens []string) {
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

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Basic" {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "invalid_authorization_format",
			Message: "Authorization header must be in format 'Basic <credentials>'",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	// Decode base64 credentials
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "invalid_authorization_format",
			Message: "Invalid base64 encoding in Authorization header",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	credentials := string(decoded)
	if !isValidToken(credentials, validTokens) {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "invalid_credentials",
			Message: "Invalid username or password",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	c.Set("auth_credentials", credentials)
}

// validateAPIKeyAuth validates API key authentication
// API key can be provided in header "X-API-Key" or query parameter "api_key"
func validateAPIKeyAuth(c *gin.Context, validTokens []string) {
	var apiKey string

	// Try X-API-Key header first
	apiKey = c.GetHeader("X-API-Key")
	if apiKey == "" {
		// Try query parameter
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "missing_api_key",
			Message: "API key is required in X-API-Key header or api_key query parameter",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	if !isValidToken(apiKey, validTokens) {
		c.JSON(http.StatusUnauthorized, types.ErrorResponse{
			Error:   "invalid_api_key",
			Message: "Invalid or expired API key",
			Code:    http.StatusUnauthorized,
		})
		c.Abort()
		return
	}

	c.Set("api_key", apiKey)
}

// isValidToken checks if the provided token is valid
func isValidToken(token string, validTokens []string) bool {
	for _, validToken := range validTokens {
		if token == validToken {
			return true
		}
	}
	return false
}
