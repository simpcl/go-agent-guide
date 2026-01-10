package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agent-guide/go-x402-facilitator/pkg/client"
	facilitatorTypes "github.com/agent-guide/go-x402-facilitator/pkg/types"
	"github.com/agent-guide/go-x402-facilitator/pkg/utils"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ChainNetwork    = "localhost"
	ChainID         = uint64(1337)
	ChainRPC        = "http://127.0.0.1:8545"
	TokenContract   = "0xBA32c2Ee180e743cCe34CbbC86cb79278C116CEb"
	TokenName       = "MyToken"
	TokenVersion    = "1"
	GatewayURL      = "http://localhost:8080"
	ResourcePath    = "/premium-data"
	BuyerPrivateKey = ""
)

func init() {
	var s string
	s = os.Getenv("CHAIN_NETWORK")
	if s != "" {
		ChainNetwork = s
	}
	s = os.Getenv("CHAIN_ID")
	if s != "" {
		var err error
		ChainID, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			fmt.Println("Error parsing ChainID:", err)
			os.Exit(-1)
		}
	}
	s = os.Getenv("CHAIN_RPC")
	if s != "" {
		ChainRPC = s
	}
	s = os.Getenv("TOKEN_CONTRACT")
	if s != "" {
		TokenContract = s
	}
	s = os.Getenv("TOKEN_NAME")
	if s != "" {
		TokenName = s
	}
	s = os.Getenv("TOKEN_VERSION")
	if s != "" {
		TokenVersion = s
	}
	s = os.Getenv("GATEWAY_URL")
	if s != "" {
		GatewayURL = s
	}
	s = os.Getenv("RESOURCE_PATH")
	if s != "" {
		ResourcePath = s
	}
	s = os.Getenv("BUYER_PRIVATE_KEY")
	if s == "" {
		log.Fatalln("ERROR: BUYER_PRIVATE_KEY environment variable is not set")
	}
	BuyerPrivateKey = s
}

// PaymentRequiredResponse represents the 402 Payment Required response
type PaymentRequiredResponse struct {
	Error               string                               `json:"error"`
	Message             string                               `json:"message"`
	Code                int                                  `json:"code"`
	PaymentRequirements facilitatorTypes.PaymentRequirements `json:"paymentRequirements"`
}

// ResourceResponse represents the response from accessing a resource
type ResourceResponse struct {
	StatusCode int
	Body       string
	Headers    http.Header
}

type Buyer struct {
	account *utils.Account
}

// NewGatewayPayer creates a new gateway payer instance
func NewBuyer() *Buyer {
	account, err := utils.NewAccountWithPrivateKey(ChainRPC, TokenContract, BuyerPrivateKey)
	if err != nil {
		log.Fatalf("ERROR: failed to create payer account: %v", err)
	}
	return &Buyer{account: account}
}

func (b *Buyer) PrintAccountInfo() {
	b.account.PrintAccountInfo("Buyer")
}

// AccessResourceGateway accesses a resource through the gateway with X402 payment
func (b *Buyer) AccessResourceGateway(resourcePath string) error {
	fmt.Println("\n=== Payment Process Beginning ===")
	// Step 1: Request resource without payment (expect 402)
	fmt.Printf("\n[Step 1] Requesting resource: %s\n", resourcePath)
	paymentReq, err := b.requestResourceWithoutPayment(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to get payment requirements: %w", err)
	}

	fmt.Printf("✅ Received payment requirements:\n")
	fmt.Printf("   Scheme: %s\n", paymentReq.Scheme)
	fmt.Printf("   Network: %s\n", paymentReq.Network)
	fmt.Printf("   PayTo: %s\n", paymentReq.PayTo)
	fmt.Printf("   MaxAmountRequired: %s\n", paymentReq.MaxAmountRequired)
	fmt.Printf("   Resource: %s\n", paymentReq.Resource)
	fmt.Printf("   Description: %s\n", paymentReq.Description)

	// Step 2: Create payment payload
	fmt.Println("Creating payment payload...")
	var validDuration int64 = 300
	now := time.Now().Unix()
	validAfter := now - 600000
	validBefore := now + validDuration
	// Generate nonce
	nonce := fmt.Sprintf(
		"0x%x",
		crypto.Keccak256Hash([]byte(fmt.Sprintf("%d-%s-%s", now, b.account.WalletAddress.Hex(), paymentReq.PayTo))).Hex(),
	)
	fmt.Printf("Nonce: %s\n", nonce)
	fmt.Println("\n[Step 2] Creating payment payload...")
	payload, err := client.CreatePaymentPayload(
		paymentReq,
		b.account.PrivateKey,
		validAfter,
		validBefore,
		ChainID,
		nonce,
	)
	if err != nil {
		return fmt.Errorf("failed to create payment payload: %w", err)
	}

	// Serialize payment payload to JSON for X-Payment header
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payment payload: %w", err)
	}

	fmt.Printf("✅ Payment payload created\n")

	// Step 3: Request resource with payment
	fmt.Println("\n[Step 3] Requesting resource with payment...")
	resourceResponse, err := b.requestResourceWithPayment(resourcePath, string(payloadJSON))
	if err != nil {
		return fmt.Errorf("failed to access resource with payment: %w", err)
	}

	fmt.Printf("✅ Resource accessed successfully!\n")
	fmt.Printf("   Response status: %d\n", resourceResponse.StatusCode)
	fmt.Printf("   Response body length: %d bytes\n", len(resourceResponse.Body))

	if len(resourceResponse.Body) > 0 {
		fmt.Printf("\n   Response preview (first 500 chars):\n")
		preview := resourceResponse.Body
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Printf("   %s\n", preview)
	}

	fmt.Println("\n=== Payment Process Complete ===")

	return nil
}

// requestResourceWithoutPayment requests a resource without payment header
// Returns payment requirements from 402 response
func (b *Buyer) requestResourceWithoutPayment(resourcePath string) (*facilitatorTypes.PaymentRequirements, error) {
	// Ensure resource path starts with /
	if !strings.HasPrefix(resourcePath, "/") {
		resourcePath = "/" + resourcePath
	}

	// Gateway API endpoint is /api/{path}
	url := fmt.Sprintf("%s/api%s", GatewayURL, resourcePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusPaymentRequired {
		return nil, fmt.Errorf("expected 402 Payment Required, got %d: %s", resp.StatusCode, string(body))
	}

	var paymentResp PaymentRequiredResponse
	if err := json.Unmarshal(body, &paymentResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment requirements: %w", err)
	}

	return &paymentResp.PaymentRequirements, nil
}

// requestResourceWithPayment requests a resource with X-Payment header
func (b *Buyer) requestResourceWithPayment(resourcePath string, paymentPayloadJSON string) (*ResourceResponse, error) {
	// Ensure resource path starts with /
	if !strings.HasPrefix(resourcePath, "/") {
		resourcePath = "/" + resourcePath
	}

	// Gateway API endpoint is /api/{path}
	url := fmt.Sprintf("%s/api%s", GatewayURL, resourcePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set X-Payment header
	req.Header.Set("X-Payment", paymentPayloadJSON)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second} // Longer timeout for payment processing
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return &ResourceResponse{
		StatusCode: resp.StatusCode,
		Body:       string(body),
		Headers:    resp.Header,
	}, nil
}

func main() {
	fmt.Println("=== X402 Gateway Payment Demo ===")
	fmt.Println()

	buyer := NewBuyer()

	buyer.PrintAccountInfo()

	if err := buyer.AccessResourceGateway(ResourcePath); err != nil {
		log.Fatalf("Payment failed: %v", err)
	}

	buyer.PrintAccountInfo()
}
