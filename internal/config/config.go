package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	GatewayServer GatewayServerConfig `mapstructure:"gateway_server"`
	AdminServer   AdminServerConfig   `mapstructure:"admin_server"`
	Endpoints     []EndpointConfig    `mapstructure:"endpoints"`
	Facilitator   FacilitatorConfig   `mapstructure:"facilitator"`
	Auth          AuthConfig          `mapstructure:"auth"`
}

// GatewayServerConfig represents gateway HTTP server configuration
type GatewayServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// AdminServerConfig represents admin HTTP server configuration
type AdminServerConfig struct {
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	MetricsEnabled bool          `mapstructure:"metrics_enabled"`
	LogLevel       string        `mapstructure:"log_level"`
	LogFormat      string        `mapstructure:"log_format"`
}

// ChainNetwork represents a blockchain network configuration
type ChainNetwork struct {
	Name          string `mapstructure:"name"`
	RPC           string `mapstructure:"rpc"`
	ID            uint64 `mapstructure:"id"`
	TokenAddress  string `mapstructure:"token_address"`
	TokenName     string `mapstructure:"token_name"`
	TokenVersion  string `mapstructure:"token_version"`
	TokenDecimals int64  `mapstructure:"token_decimals"`
	TokenType     string `mapstructure:"token_type"`
}

// FacilitatorConfig represents X402 facilitator configuration
type FacilitatorConfig struct {
	PrivateKey        string         `mapstructure:"private_key"`
	GasLimit          uint64         `mapstructure:"gas_limit"`
	GasPrice          string         `mapstructure:"gas_price"`
	X402Version       int            `mapstructure:"x402Version"`
	SupportedSchemes  []string       `mapstructure:"supported_schemes"`
	SupportedNetworks []string       `mapstructure:"supported_networks"`
	ChainNetworks     []ChainNetwork `mapstructure:"chain_networks"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	APIKeys     []string `mapstructure:"api_keys"`
	JWTSecret   string   `mapstructure:"jwt_secret"`
	RequireAuth bool     `mapstructure:"require_auth"`
}

// EndpointAuthConfig represents authentication configuration for an endpoint
type EndpointAuthConfig struct {
	Type  string `mapstructure:"type"`  // e.g., "bearer"
	Token string `mapstructure:"token"` // token value
}

// X402BuyerConfig represents X402 buyer payment configuration
type X402BuyerConfig struct {
	Network           string `mapstructure:"network"`
	PayTo             string `mapstructure:"payTo"`
	MaxAmountRequired string `mapstructure:"maxAmountRequired"`
}

// X402SellerConfig represents X402 seller payment configuration
type X402SellerConfig struct {
	Network           string `mapstructure:"network"`
	PayTo             string `mapstructure:"payTo"`
	MaxAmountRequired string `mapstructure:"maxAmountRequired"`
}

// EndpointConfig represents an endpoint configuration
type EndpointConfig struct {
	Endpoint    string              `mapstructure:"endpoint"`
	Description string              `mapstructure:"description"`
	Type        string              `mapstructure:"type"`
	Middlewares []string            `mapstructure:"middlewares"`
	Auth        *EndpointAuthConfig `mapstructure:"auth,omitempty"`
	X402Buyer   *X402BuyerConfig    `mapstructure:"x402-buyer,omitempty"`
	X402Seller  *X402SellerConfig   `mapstructure:"x402-seller,omitempty"`
	TargetURL   string              `mapstructure:"targetUrl"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig(configPath string) (*Config, error) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/agent-guide")
		viper.AddConfigPath("$HOME/.agent-guide")
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("AGENTGUIDE")
	viper.AutomaticEnv()

	// Set environment variable key replacer to handle underscores
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set default values
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Gateway server defaults
	viper.SetDefault("gateway_server.host", "0.0.0.0")
	viper.SetDefault("gateway_server.port", 8080)
	viper.SetDefault("gateway_server.read_timeout", "30s")
	viper.SetDefault("gateway_server.write_timeout", "30s")
	viper.SetDefault("gateway_server.idle_timeout", "120s")

	// Admin server defaults
	viper.SetDefault("admin_server.host", "0.0.0.0")
	viper.SetDefault("admin_server.port", 8081)
	viper.SetDefault("admin_server.read_timeout", "30s")
	viper.SetDefault("admin_server.write_timeout", "30s")
	viper.SetDefault("admin_server.idle_timeout", "120s")
	viper.SetDefault("admin_server.metrics_enabled", true)
	viper.SetDefault("admin_server.log_level", "info")
	viper.SetDefault("admin_server.log_format", "json")

	// Facilitator defaults
	viper.SetDefault("facilitator.private_key", "")
	viper.SetDefault("facilitator.gas_limit", 100000)
	viper.SetDefault("facilitator.gas_price", "")
	viper.SetDefault("facilitator.x402Version", 1)
	viper.SetDefault("facilitator.supported_schemes", []string{"exact"})
	viper.SetDefault("facilitator.supported_networks", []string{})
	viper.SetDefault("facilitator.chain_networks", []ChainNetwork{})

	// Auth defaults
	viper.SetDefault("auth.enabled", true)
	viper.SetDefault("auth.require_auth", false)
	viper.SetDefault("auth.jwt_secret", "change-this-secret-key")
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate gateway server configuration
	if config.GatewayServer.Port <= 0 || config.GatewayServer.Port > 65535 {
		return fmt.Errorf("invalid gateway server port: %d", config.GatewayServer.Port)
	}

	// Validate admin server configuration
	if config.AdminServer.Port <= 0 || config.AdminServer.Port > 65535 {
		return fmt.Errorf("invalid admin server port: %d", config.AdminServer.Port)
	}

	// Validate log level for admin server
	validLogLevels := map[string]bool{
		"trace": true, "debug": true, "info": true,
		"warn": true, "error": true, "fatal": true, "panic": true,
	}
	if !validLogLevels[config.AdminServer.LogLevel] {
		return fmt.Errorf("invalid admin server log level: %s", config.AdminServer.LogLevel)
	}

	// Validate facilitator configuration
	if len(config.Facilitator.ChainNetworks) == 0 {
		return fmt.Errorf("at least one chain network must be configured")
	}

	// Validate each chain network
	networkNames := make(map[string]bool)
	for i, network := range config.Facilitator.ChainNetworks {
		if network.Name == "" {
			return fmt.Errorf("chain network at index %d: name is required", i)
		}
		if networkNames[network.Name] {
			return fmt.Errorf("duplicate chain network name: %s", network.Name)
		}
		networkNames[network.Name] = true

		if network.RPC == "" {
			return fmt.Errorf("chain network %s: rpc is required", network.Name)
		}
		if network.ID == 0 {
			return fmt.Errorf("chain network %s: id must be greater than 0", network.Name)
		}
		if network.TokenAddress == "" {
			return fmt.Errorf("chain network %s: token_address is required", network.Name)
		}
		if network.TokenName == "" {
			return fmt.Errorf("chain network %s: token_name is required", network.Name)
		}
		if network.TokenDecimals < 0 {
			return fmt.Errorf("chain network %s: token_decimals must be non-negative", network.Name)
		}
	}

	// Validate auth configuration
	if config.Auth.Enabled && len(config.Auth.APIKeys) == 0 && config.Auth.RequireAuth {
		return fmt.Errorf("authentication enabled but no API keys configured")
	}

	return nil
}

// ToFacilitatorConfig converts gateway config to facilitator config
func (c *FacilitatorConfig) ToFacilitatorConfig() map[string]interface{} {
	return map[string]interface{}{
		"PrivateKey":        c.PrivateKey,
		"GasLimit":          c.GasLimit,
		"GasPrice":          c.GasPrice,
		"SupportedSchemes":  c.SupportedSchemes,
		"SupportedNetworks": c.SupportedNetworks,
		"ChainNetworks":     c.ChainNetworks,
	}
}
