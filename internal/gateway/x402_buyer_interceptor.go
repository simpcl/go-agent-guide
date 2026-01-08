package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go-agent-guide/internal/config"
	"github.com/agent-guide/go-x402-facilitator/pkg/client"
	"github.com/agent-guide/go-x402-facilitator/pkg/types"
	"github.com/agent-guide/go-x402-facilitator/pkg/utils"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
)

// PaymentRequiredResponse represents the 402 Payment Required response
type paymentRequiredResponse struct {
	Error               string                    `json:"error"`
	Message             string                    `json:"message"`
	Code                int                       `json:"code"`
	PaymentRequirements types.PaymentRequirements `json:"paymentRequirements"`
}

// findChainNetwork finds a chain network configuration by name
func findChainNetwork(facilitatorConfig *config.FacilitatorConfig, networkName string) *config.ChainNetwork {
	for i := range facilitatorConfig.ChainNetworks {
		if facilitatorConfig.ChainNetworks[i].Name == networkName {
			return &facilitatorConfig.ChainNetworks[i]
		}
	}
	return nil
}

// createPaymentPayload creates a payment payload using the configured private key
func createPaymentPayload(
	facilitatorConfig *config.FacilitatorConfig,
	requirements *types.PaymentRequirements,
) (*types.PaymentPayload, error) {
	// Get chain ID from chain_networks
	chainNetwork := findChainNetwork(facilitatorConfig, requirements.Network)
	if chainNetwork == nil {
		return nil, fmt.Errorf("chain network %s not found in configuration", requirements.Network)
	}
	chainID := chainNetwork.ID

	// Create account from private key
	account, err := utils.NewAccountWithPrivateKey(chainNetwork.RPC, requirements.Asset, facilitatorConfig.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create web3 account: %w", err)
	}

	// Generate payment payload
	var validDuration int64 = 300
	now := time.Now().Unix()
	validAfter := now - 600000
	validBefore := now + validDuration

	// Generate nonce
	nonce := fmt.Sprintf(
		"0x%x",
		crypto.Keccak256Hash([]byte(fmt.Sprintf("%d-%s-%s", now, account.WalletAddress.Hex(), requirements.PayTo))).Hex(),
	)

	return client.CreatePaymentPayload(
		requirements,
		account,
		validAfter,
		validBefore,
		chainID,
		nonce,
	)
}

func X402BuyerInterceptor(facilitatorConfig *config.FacilitatorConfig) InterceptorFunc {

	return func(capture *ResponseCapture, arp *AgentReverseProxy) bool {
		if capture.statusCode != http.StatusPaymentRequired {
			return false
		}

		log.Info().Msg("Received 402 Payment Required, attempting automatic payment")

		c := arp.ginContext
		targetURL := arp.targetURL

		// Parse payment requirements from response body
		var paymentResp paymentRequiredResponse
		if err := json.Unmarshal(capture.body.Bytes(), &paymentResp); err != nil {
			log.Error().Err(err).Msg("Failed to parse 402 response")
			// Return the original 402 response
			capture.flush()
			return true
		}

		// Create payment payload
		paymentPayload, err := createPaymentPayload(facilitatorConfig, &paymentResp.PaymentRequirements)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create payment payload")
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Error:   "payment_creation_failed",
				Message: fmt.Sprintf("Failed to create payment: %s", err.Error()),
				Code:    http.StatusInternalServerError,
			})
			return true
		}

		// Serialize payment payload to JSON
		paymentJSON, err := json.Marshal(paymentPayload)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal payment payload")
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Error:   "payment_serialization_failed",
				Message: fmt.Sprintf("Failed to serialize payment: %s", err.Error()),
				Code:    http.StatusInternalServerError,
			})
			return true
		}

		log.Info().Msg("Payment payload created, retrying request with payment")

		// Create a new request with X-Payment header
		// We need to recreate the request body if it exists
		var bodyReader io.Reader
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				bodyReader = bytes.NewReader(bodyBytes)
			}
		}

		retryReq, err := http.NewRequestWithContext(
			c.Request.Context(),
			c.Request.Method,
			targetURL.String(),
			bodyReader,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create retry request")
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Error:   "retry_request_failed",
				Message: fmt.Sprintf("Failed to create retry request: %s", err.Error()),
				Code:    http.StatusInternalServerError,
			})
			return true
		}

		// Add X-Payment header
		retryReq.Header.Set("X-Payment", string(paymentJSON))

		retryProxy := NewAgentReverseProxy(c, targetURL)

		// Execute the retry request directly to the original writer
		retryProxy.ServeHTTP(c.Writer, retryReq)
		return true
	}
}
