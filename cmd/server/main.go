package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/bear-san/haproxy-configurator/internal/config"
	"github.com/bear-san/haproxy-configurator/internal/logger"
	"github.com/bear-san/haproxy-configurator/internal/server"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port        int
	listenAddr  string
	configFile  string
	development bool
)

var rootCmd = &cobra.Command{
	Use:   "haproxy-configurator",
	Short: "HAProxy Configurator gRPC server",
	Long: `HAProxy Configurator is a gRPC server that manages HAProxy configuration
through the HAProxy Data Plane API. It also supports optional Netplan integration
for network configuration management.

Configuration:
  Use the -f/--config flag to specify a unified configuration file containing
  both HAProxy and Netplan settings.`,
	Run: runServer,
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 50051, "The server port")
	rootCmd.Flags().StringVarP(&listenAddr, "listen", "l", "0.0.0.0", "The server listen address")
	rootCmd.Flags().StringVarP(&configFile, "config", "f", "", "Path to the unified configuration file (required)")
	rootCmd.Flags().BoolVarP(&development, "development", "d", false, "Enable development mode logging")

	// Make config flag required
	if err := rootCmd.MarkFlagRequired("config"); err != nil {
		panic(fmt.Sprintf("Failed to mark config flag as required: %v", err))
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Initialize logger
	if err := logger.InitLogger(development); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// Construct the listen address
	listenAddress := fmt.Sprintf("%s:%d", listenAddr, port)
	
	logger.GetLogger().Info("Starting HAProxy Configurator gRPC server",
		zap.String("listen_address", listenAddress),
		zap.String("config_file", configFile),
		zap.Bool("development_mode", development))

	// Create a TCP listener on the specified address and port
	lis, err := net.Listen("tcp", listenAddress)
	if err != nil {
		logger.GetLogger().Fatal("Failed to listen",
			zap.String("listen_address", listenAddress),
			zap.Error(err))
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Load unified configuration file
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.GetLogger().Fatal("Failed to load configuration file",
			zap.String("config_file", configFile),
			zap.Error(err))
	}

	// Validate configuration
	if err := cfg.ValidateConfig(); err != nil {
		logger.GetLogger().Fatal("Invalid configuration",
			zap.String("config_file", configFile),
			zap.Error(err))
	}

	logger.GetLogger().Info("Loaded unified configuration",
		zap.String("config_file", configFile),
		zap.String("haproxy_url", cfg.HAProxy.APIURL),
		zap.String("haproxy_username", cfg.HAProxy.Username),
		zap.Bool("netplan_enabled", cfg.HasNetplanIntegration()))

	// Create and register the HAProxy manager service
	haproxyService := server.NewHAProxyManagerServerWithConfig(cfg)

	pb.RegisterHAProxyManagerServiceServer(s, haproxyService)

	// Enable reflection for development/debugging
	reflection.Register(s)

	logger.GetLogger().Info("HAProxy Configurator gRPC server ready",
		zap.String("listen_address", listenAddress),
		zap.String("example_command", fmt.Sprintf("grpcurl -plaintext localhost:%d list", port)))

	// Start serving
	if err := s.Serve(lis); err != nil {
		logger.GetLogger().Fatal("Failed to serve",
			zap.Error(err))
	}
}