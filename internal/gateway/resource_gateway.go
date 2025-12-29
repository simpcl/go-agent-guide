package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"go-agent-guide/internal/config"
	"go-x402-facilitator/pkg/client"
	"go-x402-facilitator/pkg/facilitator"
	"go-x402-facilitator/pkg/types"
	"go-x402-facilitator/pkg/utils"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// X402Config represents X402 payment configuration
type X402Config struct {
	X402Version       int    `json:"x402Version"`
	Scheme            string `json:"scheme"`
	Network           string `json:"network"`
	Resource          string `json:"resource"`
	Description       string `json:"description"`
	MaxAmountRequired string `json:"maxAmountRequired"`
	PayTo             string `json:"payTo"`
	AssetType         string `json:"assetType"`
	Asset             string `json:"asset"`
	TokenName         string `json:"tokenName"`
	TokenVersion      string `json:"tokenVersion"`
}

// AuthConfig represents authentication configuration for a resource
type AuthConfig struct {
	Type  string `json:"type"`  // e.g., "bearer"
	Token string `json:"token"` // token value
}

// ResourceConfig represents a resource configuration loaded from JSON
type ResourceConfig struct {
	Resource    string      `json:"resource"`    // API endpoint prefix
	Type        string      `json:"type"`        // e.g., "http"
	Middlewares []string    `json:"middlewares"` // List of middleware names to apply (e.g., ["auth", "x402"])
	Auth        *AuthConfig `json:"auth,omitempty"`
	X402        *X402Config `json:"x402,omitempty"`
	TargetURL   string      `json:"targetUrl"` // The actual backend URL to proxy to
}

// ResourcesList represents the structure of the resources JSON file
type ResourcesList struct {
	Resources []ResourceConfig `json:"resources"`
}

// ResourceGateway handles resource gateway operations
type ResourceGateway struct {
	facilitator    facilitator.PaymentFacilitator
	cfg            *config.Config
	resources      map[string]*ResourceConfig // Map of resource path to config
	resourcesMutex sync.RWMutex
	resourcesFile  string
	lastLoadTime   time.Time
}

// NewResourceGateway creates a new resource gateway
func NewResourceGateway(f facilitator.PaymentFacilitator, cfg *config.Config) *ResourceGateway {
	resourcesFile := cfg.Server.ResourcesFile
	if resourcesFile == "" {
		resourcesFile = "resources.json" // Default path
	}

	gateway := &ResourceGateway{
		facilitator:   f,
		cfg:           cfg,
		resources:     make(map[string]*ResourceConfig),
		resourcesFile: resourcesFile,
	}

	// Load resources on startup
	if err := gateway.loadResources(); err != nil {
		log.Warn().Err(err).Msg("Failed to load resources on startup, will retry on first request")
	}

	return gateway
}

// DiscoverResources returns discovered resources from loaded configuration
func (g *ResourceGateway) DiscoverResources(ctx context.Context, resourceType string, limit, offset int) (*types.DiscoveryResponse, error) {
	// Reload resources if needed
	if err := g.ReloadResourcesIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to reload resources for discovery")
	}

	g.resourcesMutex.RLock()
	defer g.resourcesMutex.RUnlock()

	// Convert resources to discovery items
	var items []types.DiscoveryItem
	for _, resource := range g.resources {
		// Filter by type if specified
		if resourceType != "" && resource.Type != resourceType {
			continue
		}

		// Convert X402Config to PaymentRequirements if x402 is configured
		var accepts []types.PaymentRequirements
		if resource.X402 != nil {
			accepts = []types.PaymentRequirements{
				{
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
				},
			}
		}

		x402Version := 0
		if resource.X402 != nil {
			x402Version = resource.X402.X402Version
		}

		items = append(items, types.DiscoveryItem{
			Resource:    resource.Resource,
			Type:        resource.Type,
			X402Version: x402Version,
			Accepts:     accepts,
		})
	}

	// Apply pagination
	start := offset
	if start > len(items) {
		start = len(items)
	}

	end := start + limit
	if end > len(items) {
		end = len(items)
	}

	var paginatedItems []types.DiscoveryItem
	if start < len(items) {
		paginatedItems = items[start:end]
	}

	return &types.DiscoveryResponse{
		X402Version: 1,
		Items:       paginatedItems,
	}, nil
}

// loadResources loads resources from the JSON file
func (g *ResourceGateway) loadResources() error {
	// Check if file exists
	if _, err := os.Stat(g.resourcesFile); os.IsNotExist(err) {
		log.Warn().Str("file", g.resourcesFile).Msg("Resources file not found, using empty resource list")
		return nil
	}

	// Read file
	data, err := os.ReadFile(g.resourcesFile)
	if err != nil {
		return fmt.Errorf("failed to read resources file: %w", err)
	}

	// Parse JSON
	var resourcesList ResourcesList
	if err := json.Unmarshal(data, &resourcesList); err != nil {
		return fmt.Errorf("failed to parse resources JSON: %w", err)
	}

	// Update resources map
	g.resourcesMutex.Lock()
	defer g.resourcesMutex.Unlock()

	g.resources = make(map[string]*ResourceConfig)
	for i := range resourcesList.Resources {
		resource := &resourcesList.Resources[i]
		// Normalize resource path (ensure it starts with /, remove trailing slash except for root)
		resourcePath := resource.Resource
		if !strings.HasPrefix(resourcePath, "/") {
			resourcePath = "/" + resourcePath
		}
		// Remove trailing slash except for root path "/"
		if resourcePath != "/" && strings.HasSuffix(resourcePath, "/") {
			resourcePath = strings.TrimSuffix(resourcePath, "/")
		}
		// Update the resource's Resource field to normalized path for consistency
		resource.Resource = resourcePath
		g.resources[resourcePath] = resource
	}

	g.lastLoadTime = time.Now()
	log.Info().
		Int("count", len(g.resources)).
		Str("file", g.resourcesFile).
		Msg("Resources loaded successfully")

	return nil
}

// ReloadResourcesIfNeeded reloads resources if the file has been modified
func (g *ResourceGateway) ReloadResourcesIfNeeded() error {
	// Check if file exists
	info, err := os.Stat(g.resourcesFile)
	if os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to reload
	}

	// Check if file was modified after last load
	if info.ModTime().After(g.lastLoadTime) {
		log.Info().Msg("Resources file modified, reloading...")
		return g.loadResources()
	}

	return nil
}

// GetAllResources returns all resource configurations
func (g *ResourceGateway) GetAllResources() []*ResourceConfig {
	g.resourcesMutex.RLock()
	defer g.resourcesMutex.RUnlock()

	resources := make([]*ResourceConfig, 0, len(g.resources))
	for _, resource := range g.resources {
		resources = append(resources, resource)
	}

	return resources
}

// FindResource finds a resource configuration by path
func (g *ResourceGateway) FindResource(path string) *ResourceConfig {
	g.resourcesMutex.RLock()
	defer g.resourcesMutex.RUnlock()

	// Try exact match first
	if resource, exists := g.resources[path]; exists {
		return resource
	}

	// Try exact match with trailing slash removed (if path has trailing slash)
	normalizedPath := strings.TrimSuffix(path, "/")
	if normalizedPath != path {
		if resource, exists := g.resources[normalizedPath]; exists {
			return resource
		}
	}

	// Try to find longest matching prefix
	var bestMatch *ResourceConfig
	var bestMatchLen int

	for resourcePath, resource := range g.resources {
		// Check if path starts with resourcePath (with or without trailing slash)
		if strings.HasPrefix(path, resourcePath) && len(resourcePath) > bestMatchLen {
			bestMatch = resource
			bestMatchLen = len(resourcePath)
		}
		// Also check normalized path
		if normalizedPath != path && strings.HasPrefix(normalizedPath, resourcePath) && len(resourcePath) > bestMatchLen {
			bestMatch = resource
			bestMatchLen = len(resourcePath)
		}
	}

	return bestMatch
}

// PaymentRequiredResponse represents the 402 Payment Required response
type paymentRequiredResponse struct {
	Error               string                    `json:"error"`
	Message             string                    `json:"message"`
	Code                int                       `json:"code"`
	PaymentRequirements types.PaymentRequirements `json:"paymentRequirements"`
}

func (g *ResourceGateway) createWeb3Account(network string, tokenContractAddr string) (*utils.Account, error) {
	// Check if private key is configured
	if g.cfg.Facilitator.PrivateKey == "" {
		return nil, fmt.Errorf("private key not configured for automatic payment")
	}

	// Get chain configuration
	chainRPC := g.cfg.Facilitator.DefaultChainRPC
	if rpc, ok := g.cfg.Facilitator.ChainRPCs[network]; ok {
		chainRPC = rpc
	}

	// Create account from private key
	return utils.NewAccountWithPrivateKey(chainRPC, tokenContractAddr, g.cfg.Facilitator.PrivateKey)
}

// createPaymentPayload creates a payment payload using the configured private key
func (g *ResourceGateway) createPaymentPayload(requirements *types.PaymentRequirements) (*types.PaymentPayload, error) {
	// Create account from private key
	account, err := g.createWeb3Account(requirements.Network, requirements.Asset)
	if err != nil {
		return nil, fmt.Errorf("failed to create web3 account: %w", err)
	}

	chainID := g.cfg.Facilitator.DefaultChainID
	if id, ok := g.cfg.Facilitator.ChainIds[requirements.Network]; ok {
		chainID = id
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

// ProxyRequest proxies the request to the target URL
func (g *ResourceGateway) ProxyRequest(c *gin.Context, resource *ResourceConfig) {
	if resource.TargetURL == "" {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "internal_error",
			Message: "Resource target URL not configured",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Parse target URL
	targetURL, err := url.Parse(resource.TargetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse{
			Error:   "internal_error",
			Message: fmt.Sprintf("Invalid target URL: %s", err.Error()),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	proxy := NewAgentReverseProxy(c, targetURL)

	// Create response capture to intercept 402 responses
	capture := NewResponseCapture(c.Writer)

	// Serve the request
	proxy.ServeHTTP(capture, c.Request)

	// Check if we got a 402 Payment Required response
	if capture.statusCode == http.StatusPaymentRequired {
		log.Info().Msg("Received 402 Payment Required, attempting automatic payment")

		// Parse payment requirements from response body
		var paymentResp paymentRequiredResponse
		if err := json.Unmarshal(capture.body.Bytes(), &paymentResp); err != nil {
			log.Error().Err(err).Msg("Failed to parse 402 response")
			// Return the original 402 response
			capture.flush()
			return
		}

		// Create payment payload
		paymentPayload, err := g.createPaymentPayload(&paymentResp.PaymentRequirements)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create payment payload")
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Error:   "payment_creation_failed",
				Message: fmt.Sprintf("Failed to create payment: %s", err.Error()),
				Code:    http.StatusInternalServerError,
			})
			return
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
			return
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
			return
		}

		// Add X-Payment header
		retryReq.Header.Set("X-Payment", string(paymentJSON))

		retryProxy := NewAgentReverseProxy(c, targetURL)

		// Execute the retry request directly to the original writer
		retryProxy.ServeHTTP(c.Writer, retryReq)
	} else {
		// Not a 402, flush the captured response
		capture.flush()
	}
}
