package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/bear-san/haproxy-configurator/internal/logger"
	"github.com/bear-san/haproxy-configurator/internal/server"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port          = flag.Int("port", 50051, "The server port")
	netplanConfig = flag.String("netplan-config", "", "Path to the Netplan configuration file (optional)")
	development   = flag.Bool("development", false, "Enable development mode logging")
)

func main() {
	flag.Parse()

	// Initialize logger
	if err := logger.InitLogger(*development); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.GetLogger().Info("Starting HAProxy Configurator gRPC server",
		zap.Int("port", *port),
		zap.String("netplan_config", *netplanConfig),
		zap.Bool("development_mode", *development))

	// Create a TCP listener on the specified port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logger.GetLogger().Fatal("Failed to listen",
			zap.Int("port", *port),
			zap.Error(err))
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Create and register the HAProxy manager service
	haproxyService := server.NewHAProxyManagerServer()

	// Configure Netplan integration if config file is provided
	if *netplanConfig != "" {
		logger.GetLogger().Info("Initializing Netplan integration",
			zap.String("config_file", *netplanConfig))
		if err := haproxyService.SetNetplanConfig(*netplanConfig); err != nil {
			logger.GetLogger().Fatal("Failed to initialize Netplan configuration",
				zap.String("config_file", *netplanConfig),
				zap.Error(err))
		}
	} else {
		logger.GetLogger().Info("Netplan integration disabled (no config file provided)")
	}

	pb.RegisterHAProxyManagerServiceServer(s, haproxyService)

	// Enable reflection for development/debugging
	reflection.Register(s)

	logger.GetLogger().Info("HAProxy Configurator gRPC server ready",
		zap.Int("port", *port),
		zap.String("example_command", fmt.Sprintf("grpcurl -plaintext localhost:%d list", *port)))

	// Start serving
	if err := s.Serve(lis); err != nil {
		logger.GetLogger().Fatal("Failed to serve",
			zap.Error(err))
	}
}
