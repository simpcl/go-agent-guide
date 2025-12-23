package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go-agent-guide/internal/gateway"
	"go-x402-facilitator/pkg/facilitator"
	"go-x402-facilitator/pkg/types"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ResourcePayMiddleware provides resource-specific payment verification middleware
// It checks resources file to determine if payment verification is required
// This is a Resource-level middleware, corresponding to ResourceAuthMiddleware
func ResourcePayMiddleware(facilitator facilitator.PaymentFacilitator, resourceGateway *gateway.ResourceGateway) gin.HandlerFunc {
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
			// Resource not found, skip payment verification (will be handled by handler)
			c.Next()
			return
		}

		// Store resource in context for handler and other middlewares to use
		// (Set it even if payment verification is not required, so handler knows resource exists)
		c.Set("resource_config", resource)

		// Check if payment middleware is required for this resource
		hasPayment := false
		for _, mw := range resource.Middlewares {
			if mw == "x402" {
				hasPayment = true
				break
			}
		}

		if !hasPayment || resource.X402 == nil {
			// No payment requirement, continue
			c.Next()
			return
		}

		// Check for X-Payment header
		paymentHeader := c.GetHeader("X-Payment")
		if paymentHeader == "" {
			// No payment provided, return 402 Payment Required
			returnPaymentRequired(c, resource)
			c.Abort()
			return
		}

		// Parse and validate payment
		if err := processPayment(c, facilitator, resource, paymentHeader); err != nil {
			log.Error().Err(err).Msg("Payment processing failed")
			c.JSON(http.StatusPaymentRequired, types.ErrorResponse{
				Error:   "payment_failed",
				Message: err.Error(),
				Code:    http.StatusPaymentRequired,
			})
			c.Abort()
			return
		}

		// Payment successful, continue to next handler
		c.Next()
	}
}

// returnPaymentRequired returns a 402 Payment Required response with payment requirements
func returnPaymentRequired(c *gin.Context, resource *gateway.ResourceConfig) {
	if resource.X402 == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "internal_error",
			Message: "Resource has no X402 payment requirements configured",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Convert X402Config to PaymentRequirements
	requirements := types.PaymentRequirements{
		Scheme:            resource.X402.Scheme,
		Network:           resource.X402.Network,
		Resource:          resource.X402.Resource,
		Description:       resource.X402.Description,
		MaxAmountRequired: resource.X402.MaxAmountRequired,
		PayTo:             resource.X402.PayTo,
		AssetType:         resource.X402.AssetType,
		Asset:             resource.X402.Asset,
		TokenName:         resource.X402.TokenName,
		TokenVersion:      resource.X402.TokenVersion,
	}

	// Return 402 with payment requirements
	c.Header("X-Payment-Required", "true")
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":               "payment_required",
		"message":             "Payment is required to access this resource",
		"code":                http.StatusPaymentRequired,
		"paymentRequirements": requirements,
	})
}

// processPayment processes the X-Payment header and verifies/settles the payment
func processPayment(c *gin.Context, facilitator facilitator.PaymentFacilitator, resource *gateway.ResourceConfig, paymentHeader string) error {
	// Parse X-Payment header (should be JSON)
	var paymentPayload types.PaymentPayload
	if err := json.Unmarshal([]byte(paymentHeader), &paymentPayload); err != nil {
		return fmt.Errorf("failed to parse X-Payment header: %w", err)
	}

	if resource.X402 == nil {
		return fmt.Errorf("resource has no X402 configuration")
	}

	// Verify scheme and network match
	if paymentPayload.Scheme != resource.X402.Scheme || paymentPayload.Network != resource.X402.Network {
		return fmt.Errorf("payment scheme/network mismatch: expected scheme=%s network=%s, got scheme=%s network=%s",
			resource.X402.Scheme, resource.X402.Network, paymentPayload.Scheme, paymentPayload.Network)
	}

	// Convert X402Config to PaymentRequirements
	requirements := types.PaymentRequirements{
		Scheme:            resource.X402.Scheme,
		Network:           resource.X402.Network,
		Resource:          resource.X402.Resource,
		Description:       resource.X402.Description,
		MaxAmountRequired: resource.X402.MaxAmountRequired,
		PayTo:             resource.X402.PayTo,
		AssetType:         resource.X402.AssetType,
		Asset:             resource.X402.Asset,
		TokenName:         resource.X402.TokenName,
		TokenVersion:      resource.X402.TokenVersion,
	}

	// Create verify request
	verifyReq := types.VerifyRequest{
		PaymentPayload:      paymentPayload,
		PaymentRequirements: requirements,
	}

	// Verify payment
	ctx := c.Request.Context()
	verifyResp, err := facilitator.Verify(ctx, &verifyReq)
	if err != nil {
		return fmt.Errorf("payment verification failed: %w", err)
	}

	if !verifyResp.IsValid {
		return fmt.Errorf("payment is invalid: %s", verifyResp.InvalidReason)
	}

	// Settle payment
	settleResp, err := facilitator.Settle(ctx, &verifyReq)
	if err != nil {
		return fmt.Errorf("payment settlement failed: %w", err)
	}

	if !settleResp.Success {
		return fmt.Errorf("payment settlement failed: %s", settleResp.ErrorReason)
	}

	log.Info().
		Str("resource", resource.Resource).
		Str("payer", settleResp.Payer).
		Str("transaction", settleResp.Transaction).
		Msg("Payment processed successfully")

	// Store payment info in context for potential use in proxy
	c.Set("payment_payer", settleResp.Payer)
	c.Set("payment_transaction", settleResp.Transaction)

	return nil
}
