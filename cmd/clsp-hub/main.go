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

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	dbPath := flag.String("db", "", "Path to database file (default: global config location)")
	flag.Parse()

	if len(flag.Args()) > 0 && flag.Args()[0] == "init" {
		doInit(*dbPath)
		return
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
