package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mattd/clsp/internal/hub"
	"github.com/mattd/clsp/internal/paths"
)

func doInit(dbPath string) {
	if dbPath == "" {
		dbPath = paths.HubDBPath
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	server, err := hub.NewServer(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	server.Shutdown() // Close DB connection
	fmt.Printf("Initialization successful! Directory '%s' and database '%s' are ready.\n", dir, dbPath)
}

func doConfig(dbPath string, timeout, expiry, rateLimit int) {
	if dbPath == "" {
		dbPath = paths.HubDBPath
	}

	server, err := hub.NewServer(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer server.Shutdown()

	// Update configuration
	if timeout > 0 {
		server.SetTimeout(time.Duration(timeout) * time.Second)
	}
	if expiry > 0 {
		server.SetMessageExpiry(time.Duration(expiry) * time.Hour)
	}
	if rateLimit > 0 {
		server.SetRateLimit(rateLimit)
	}

	fmt.Println("Hub configuration updated successfully!")
}

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	dbPath := flag.String("db", "", "Path to database file (default: global config location)")
	flag.Parse()

	// Handle subcommands
	if len(flag.Args()) > 0 {
		switch flag.Args()[0] {
		case "init":
			doInit(*dbPath)
			return
		case "config":
			configCmd := flag.NewFlagSet("config", flag.ExitOnError)
			timeout := configCmd.Int("timeout", 0, "Set hub timeout in seconds")
			expiry := configCmd.Int("expiry", 0, "Set message expiry in hours")
			rateLimit := configCmd.Int("rate-limit", 0, "Set rate limit (messages per minute)")
			configCmd.Parse(flag.Args()[1:])
			doConfig(*dbPath, *timeout, *expiry, *rateLimit)
			return
		default:
			fmt.Printf("Unknown command: %s\n", flag.Args()[0])
			fmt.Println("Available commands:")
			fmt.Println("  init                    Initialize hub database")
			fmt.Println("  config                  Configure hub settings")
			fmt.Println("    --timeout <seconds>   Set hub timeout")
			fmt.Println("    --expiry <hours>      Set message expiry")
			fmt.Println("    --rate-limit <count>  Set rate limit")
			return
		}
	}

	// Create server with database path
	server, err := hub.NewServer(*dbPath)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Set the port
	server.SetPort(*port)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("CLSP Hub server starting on port %d...\n", *port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-sigChan
	fmt.Println("\nShutting down hub server...")
	server.Shutdown()
}
