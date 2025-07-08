package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/bear-san/haproxy-configurator/internal/logger"
	"github.com/bear-san/haproxy-configurator/internal/server"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port          int
	netplanConfig string
	development   bool
)

var rootCmd = &cobra.Command{
	Use:   "haproxy-configurator",
	Short: "HAProxy Configurator gRPC server",
	Long: `HAProxy Configurator is a gRPC server that manages HAProxy configuration
through the HAProxy Data Plane API. It also supports optional Netplan integration
for network configuration management.

Environment Variables:
  HAPROXY_API_URL         HAProxy Data Plane API URL (default: http://localhost:5555)
  HAPROXY_API_USERNAME    API username for authentication (default: admin)
  HAPROXY_API_PASSWORD    API password for authentication (default: admin)`,
	Run: runServer,
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 50051, "The server port")
	rootCmd.Flags().StringVarP(&netplanConfig, "netplan-config", "n", "", "Path to the Netplan configuration file (optional)")
	rootCmd.Flags().BoolVarP(&development, "development", "d", false, "Enable development mode logging")
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

	logger.GetLogger().Info("Starting HAProxy Configurator gRPC server",
		zap.Int("port", port),
		zap.String("netplan_config", netplanConfig),
		zap.Bool("development_mode", development))

	// Create a TCP listener on the specified port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logger.GetLogger().Fatal("Failed to listen",
			zap.Int("port", port),
			zap.Error(err))
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Create and register the HAProxy manager service
	haproxyService := server.NewHAProxyManagerServer()

	// Configure Netplan integration if config file is provided
	if netplanConfig != "" {
		logger.GetLogger().Info("Initializing Netplan integration",
			zap.String("config_file", netplanConfig))
		if err := haproxyService.SetNetplanConfig(netplanConfig); err != nil {
			logger.GetLogger().Fatal("Failed to initialize Netplan configuration",
				zap.String("config_file", netplanConfig),
				zap.Error(err))
		}
	} else {
		logger.GetLogger().Info("Netplan integration disabled (no config file provided)")
	}

	pb.RegisterHAProxyManagerServiceServer(s, haproxyService)

	// Enable reflection for development/debugging
	reflection.Register(s)

	logger.GetLogger().Info("HAProxy Configurator gRPC server ready",
		zap.Int("port", port),
		zap.String("example_command", fmt.Sprintf("grpcurl -plaintext localhost:%d list", port)))

	// Start serving
	if err := s.Serve(lis); err != nil {
		logger.GetLogger().Fatal("Failed to serve",
			zap.Error(err))
	}
}