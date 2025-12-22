package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-agent-guide/internal/config"
	"go-agent-guide/internal/server"
	"go-x402-facilitator/pkg/facilitator"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	configPath = flag.String("config", "", "Path to configuration file")
	version    = flag.Bool("version", false, "Show version information")
)

const (
	AppName    = "agent-guide"
	AppVersion = "1.0.0"
	AppDesc    = "Production-ready resource gateway with X402 payment integration"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s - %s\n", AppName, AppVersion, AppDesc)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize logger
	setupLogger(cfg)

	log.Info().
		Str("version", AppVersion).
		Msg("Starting Resource Gateway")

	// Create facilitator instance
	// Build networks map from configuration
	networks := make(map[string]facilitator.NetworkConfig)

	// Add default network if configured
	if cfg.Facilitator.DefaultChainNetwork != "" {
		networks[cfg.Facilitator.DefaultChainNetwork] = facilitator.NetworkConfig{
			ChainRPC:      cfg.Facilitator.DefaultChainRPC,
			ChainID:       cfg.Facilitator.DefaultChainID,
			TokenAddress:  cfg.Facilitator.DefaultTokenAddress,
			TokenName:     cfg.Facilitator.DefaultTokenName,
			TokenVersion:  cfg.Facilitator.DefaultTokenVersion,
			TokenDecimals: cfg.Facilitator.DefaultTokenDecimals,
			GasLimit:      cfg.Facilitator.GasLimit,
			GasPrice:      cfg.Facilitator.GasPrice,
		}
	}

	// Add additional networks from chain_ids, chain_rpcs, and token_contracts
	for networkName, chainID := range cfg.Facilitator.ChainIds {
		chainRPC := cfg.Facilitator.ChainRPCs[networkName]
		tokenAddress := cfg.Facilitator.TokenContracts[networkName]

		// Use default values if not specified
		tokenName := cfg.Facilitator.DefaultTokenName
		tokenVersion := cfg.Facilitator.DefaultTokenVersion
		tokenDecimals := cfg.Facilitator.DefaultTokenDecimals
		gasLimit := cfg.Facilitator.GasLimit
		gasPrice := cfg.Facilitator.GasPrice

		networks[networkName] = facilitator.NetworkConfig{
			ChainRPC:      chainRPC,
			ChainID:       chainID,
			TokenAddress:  tokenAddress,
			TokenName:     tokenName,
			TokenVersion:  tokenVersion,
			TokenDecimals: tokenDecimals,
			GasLimit:      gasLimit,
			GasPrice:      gasPrice,
		}
	}

	// Get supported scheme (use first one if multiple)
	supportedScheme := "exact" // default
	if len(cfg.Facilitator.SupportedSchemes) > 0 {
		supportedScheme = cfg.Facilitator.SupportedSchemes[0]
	}

	facilitatorConfig := &facilitator.FacilitatorConfig{
		Networks:        networks,
		PrivateKey:      cfg.Facilitator.PrivateKey,
		SupportedScheme: supportedScheme,
	}

	f, err := facilitator.New(facilitatorConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create facilitator")
	}
	defer f.Close()

	log.Info().Msg("Facilitator initialized successfully")

	// Create API server
	server := server.NewServer(cfg, f)

	// Start metrics server if enabled
	if err := server.StartMetricsServer(); err != nil {
		log.Warn().Err(err).Msg("Failed to start metrics server")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Error().Err(err).Msg("Server failed to start")
			cancel()
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Info().Msg("Received shutdown signal")
	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down")
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info().Msg("Shutting down gracefully...")

	if err := server.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during server shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Shutdown completed successfully")
}

// setupLogger configures the global logger
func setupLogger(cfg *config.Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Monitoring.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output format
	if cfg.Monitoring.LogFormat == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = log.With().Timestamp().Logger()
	}

	// Add default context fields
	log.Logger = log.Logger.With().
		Str("service", AppName).
		Str("version", AppVersion).
		Logger()
}
