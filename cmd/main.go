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
	"github.com/agent-guide/go-x402-facilitator/pkg/facilitator"

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
	// Build networks map from chain_networks configuration
	networks := make(map[string]facilitator.NetworkConfig)

	// Add networks from chain_networks array
	for _, chainNetwork := range cfg.Facilitator.ChainNetworks {
		networks[chainNetwork.Name] = facilitator.NetworkConfig{
			ChainRPC:      chainNetwork.RPC,
			ChainID:       chainNetwork.ID,
			TokenAddress:  chainNetwork.TokenAddress,
			TokenName:     chainNetwork.TokenName,
			TokenVersion:  chainNetwork.TokenVersion,
			TokenDecimals: chainNetwork.TokenDecimals,
			GasLimit:      cfg.Facilitator.GasLimit,
			GasPrice:      cfg.Facilitator.GasPrice,
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

	// Create gateway server
	gatewayServer := server.NewGatewayServer(cfg, f)

	// Create admin server
	adminServer := server.NewAdminServer(cfg, f)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start gateway server in a goroutine
	go func() {
		if err := gatewayServer.Start(); err != nil {
			log.Error().Err(err).Msg("Gateway server failed to start")
			cancel()
		}
	}()

	// Start admin server in a goroutine
	go func() {
		if err := adminServer.Start(); err != nil {
			log.Error().Err(err).Msg("Admin server failed to start")
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

	// Stop both servers
	if err := gatewayServer.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during gateway server shutdown")
		os.Exit(1)
	}

	if err := adminServer.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during admin server shutdown")
		os.Exit(1)
	}

	log.Info().Msg("Shutdown completed successfully")
}

// setupLogger configures the global logger
func setupLogger(cfg *config.Config) {
	// Set log level from admin server config
	logLevel := cfg.AdminServer.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output format from admin server config
	logFormat := cfg.AdminServer.LogFormat
	if logFormat == "" {
		logFormat = "json"
	}
	if logFormat == "console" {
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
