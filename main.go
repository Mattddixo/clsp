package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattd/clsp/internal/hub"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	dbPath := flag.String("db", ".clsp/hub.db", "Path to database file")
	flag.Parse()

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
