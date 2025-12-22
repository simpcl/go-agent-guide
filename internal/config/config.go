package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Facilitator FacilitatorConfig `mapstructure:"facilitator"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Monitoring  MonitoringConfig  `mapstructure:"monitoring"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	ReadTimeout   time.Duration `mapstructure:"read_timeout"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`
	ResourcesFile string        `mapstructure:"resources_file"`
}

// FacilitatorConfig represents X402 facilitator configuration
type FacilitatorConfig struct {
	DefaultChainNetwork  string            `mapstructure:"default_chain_network"`
	DefaultChainRPC      string            `mapstructure:"default_chain_rpc"`
	DefaultChainID       uint64            `mapstructure:"default_chain_id"`
	DefaultTokenAddress  string            `mapstructure:"default_token_address"`
	DefaultTokenName     string            `mapstructure:"default_token_name"`
	DefaultTokenVersion  string            `mapstructure:"default_token_version"`
	DefaultTokenDecimals int64             `mapstructure:"default_token_decimals"`
	PrivateKey           string            `mapstructure:"private_key"`
	GasLimit             uint64            `mapstructure:"gas_limit"`
	GasPrice             string            `mapstructure:"gas_price"`
	SupportedSchemes     []string          `mapstructure:"supported_schemes"`
	SupportedNetworks    []string          `mapstructure:"supported_networks"`
	ChainIds             map[string]uint64 `mapstructure:"chain_ids"`
	ChainRPCs            map[string]string `mapstructure:"chain_rpcs"`
	TokenContracts       map[string]string `mapstructure:"token_contracts"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled     bool     `mapstructure:"enabled"`
	APIKeys     []string `mapstructure:"api_keys"`
	JWTSecret   string   `mapstructure:"jwt_secret"`
	RequireAuth bool     `mapstructure:"require_auth"`
}

// MonitoringConfig represents monitoring and observability configuration
type MonitoringConfig struct {
	MetricsEnabled bool   `mapstructure:"metrics_enabled"`
	MetricsPort    int    `mapstructure:"metrics_port"`
	LogLevel       string `mapstructure:"log_level"`
	LogFormat      string `mapstructure:"log_format"`
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
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.resources_file", "resources.json")

	// Facilitator defaults
	viper.SetDefault("facilitator.default_chain_network", "localhost")
	viper.SetDefault("facilitator.default_chain_rpc", "http://127.0.0.1:8545")
	viper.SetDefault("facilitator.default_chain_id", 1337)
	viper.SetDefault("facilitator.default_token_address", "")
	viper.SetDefault("facilitator.default_token_name", "MyToken")
	viper.SetDefault("facilitator.default_token_version", "1")
	viper.SetDefault("facilitator.default_token_decimals", 6)
	viper.SetDefault("facilitator.private_key", "")
	viper.SetDefault("facilitator.gas_limit", 100000)
	viper.SetDefault("facilitator.gas_price", "")
	viper.SetDefault("facilitator.supported_schemes", []string{"exact"})
	viper.SetDefault("facilitator.supported_networks", []string{})

	// Auth defaults
	viper.SetDefault("auth.enabled", true)
	viper.SetDefault("auth.require_auth", false)
	viper.SetDefault("auth.jwt_secret", "change-this-secret-key")

	// Monitoring defaults
	viper.SetDefault("monitoring.metrics_enabled", true)
	viper.SetDefault("monitoring.metrics_port", 9090)
	viper.SetDefault("monitoring.log_level", "info")
	viper.SetDefault("monitoring.log_format", "json")
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate server configuration
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Validate auth configuration
	if config.Auth.Enabled && len(config.Auth.APIKeys) == 0 && config.Auth.RequireAuth {
		return fmt.Errorf("authentication enabled but no API keys configured")
	}

	// Validate monitoring configuration
	validLogLevels := map[string]bool{
		"trace": true, "debug": true, "info": true,
		"warn": true, "error": true, "fatal": true, "panic": true,
	}
	if !validLogLevels[config.Monitoring.LogLevel] {
		return fmt.Errorf("invalid log level: %s", config.Monitoring.LogLevel)
	}

	return nil
}

// ToFacilitatorConfig converts gateway config to facilitator config
func (c *FacilitatorConfig) ToFacilitatorConfig() map[string]interface{} {
	return map[string]interface{}{
		"DefaultChainNetwork":  c.DefaultChainNetwork,
		"DefaultChainRPC":      c.DefaultChainRPC,
		"DefaultChainID":       c.DefaultChainID,
		"DefaultTokenAddress":  c.DefaultTokenAddress,
		"DefaultTokenName":     c.DefaultTokenName,
		"DefaultTokenVersion":  c.DefaultTokenVersion,
		"DefaultTokenDecimals": c.DefaultTokenDecimals,
		"PrivateKey":           c.PrivateKey,
		"GasLimit":             c.GasLimit,
		"GasPrice":             c.GasPrice,
		"SupportedSchemes":     c.SupportedSchemes,
		"SupportedNetworks":    c.SupportedNetworks,
		"ChainIds":             c.ChainIds,
		"ChainRPCs":            c.ChainRPCs,
		"TokenContracts":       c.TokenContracts,
	}
}
