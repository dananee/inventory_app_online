package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/joho/godotenv"

	"inventory_app/internal/auth"
	"inventory_app/internal/db"
	"inventory_app/internal/handlers"
	"inventory_app/internal/middleware"
	"inventory_app/router"
)

var embeddedFiles embed.FS

// getAvailableListener tries to bind to the preferred port, falls back to :0 (random OS port)
func getAvailableListener(preferredPort int) (net.Listener, int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", preferredPort))
	if err != nil {
		log.Printf("Port %d is in use, trying random port...", preferredPort)
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			log.Fatalf("Failed to bind to any port: %v", err)
		}
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, running with system environment variables.")
	}

	// Initialize PostgreSQL database
	database, err := db.InitDatabase()
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer database.Close()

	// Bind to available port
	preferredPort := 8085
	ln, port := getAvailableListener(preferredPort)

	// Build app context
	ctx := &handlers.AppContext{
		DB:   database,
		Port: port,
	}

	// Register all routes
	mux := http.NewServeMux()
	router.SetupRoutes(mux, ctx)

	// Serve embedded web frontend (if embedded)
	mux.Handle("/", http.FileServer(http.FS(embeddedFiles)))

	// Wrap mux with request logger middleware
	loggedMux := middleware.RequestLogger(mux)

	// Broadcast mDNS service for mobile discovery
	go func() {
		server, err := zeroconf.Register("InvotPOS", "_invot._tcp", "local.", port, []string{"txtv=0", "lo=1", "la=2"}, nil)
		if err != nil {
			log.Printf("mDNS registration warning: %v (non-fatal)", err)
			return
		}
		defer server.Shutdown()
		select {}
	}()

	printLocalIPs(port)

	log.Printf("🚀 Inventory server started on port %d", port)
	srv := &http.Server{
		Handler:      loggedMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := srv.Serve(ln); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// printLocalIPs prints local network addresses and QR-code-friendly activation keys
func printLocalIPs(port int) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Could not discover local IPs: %v", err)
		return
	}

	key := auth.GenerateActivationKey(time.Now(), "GO-INVENTORY")
	legacyKey := fmt.Sprintf("GO-INVENTORY-%s", time.Now().UTC().Format("01-02-06-15-04"))

	log.Println("====================================================")
	log.Printf("Activation Key (SHA-256): %s", key)
	log.Printf("Legacy Key Format: %s", legacyKey)
	log.Printf("Server Port: %d", port)
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				log.Printf("  → http://%s:%d", ipnet.IP.String(), port)
			}
		}
	}
	log.Println("====================================================")

	// Set SERVER_PORT env for other modules if needed
	_ = os.Setenv("SERVER_PORT", fmt.Sprintf("%d", port))
}
