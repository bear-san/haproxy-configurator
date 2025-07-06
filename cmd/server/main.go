package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/bear-san/haproxy-configurator/internal/server"
	pb "github.com/bear-san/haproxy-configurator/pkg/haproxy/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	port          = flag.Int("port", 50051, "The server port")
	netplanConfig = flag.String("netplan-config", "", "Path to the Netplan configuration file (optional)")
)

func main() {
	flag.Parse()

	// Create a TCP listener on the specified port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Create and register the HAProxy manager service
	haproxyService := server.NewHAProxyManagerServer()

	// Configure Netplan integration if config file is provided
	if *netplanConfig != "" {
		log.Printf("Initializing Netplan integration with config: %s", *netplanConfig)
		if err := haproxyService.SetNetplanConfig(*netplanConfig); err != nil {
			log.Fatalf("Failed to initialize Netplan configuration: %v", err)
		}
	} else {
		log.Printf("Netplan integration disabled (no config file provided)")
	}

	pb.RegisterHAProxyManagerServiceServer(s, haproxyService)

	// Enable reflection for development/debugging
	reflection.Register(s)

	log.Printf("HAProxy Configurator gRPC server listening on port %d", *port)
	log.Printf("Use grpcurl or other gRPC clients to interact with the server")
	log.Printf("Example: grpcurl -plaintext localhost:%d list", *port)

	// Start serving
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

