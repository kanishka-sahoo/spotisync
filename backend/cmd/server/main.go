package main

import (
	"flag"
	"fmt"
	"log"

	"spotisync/internal/api"
	"spotisync/internal/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start server
	server, err := api.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start the server (blocks until shutdown)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("Spotisync server stopped")
}
