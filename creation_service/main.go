package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"brainbot/creation_service/app/api"
	"brainbot/creation_service/app/config"
	"brainbot/creation_service/app/services"
)

const (
	// ServiceAccountPath is the path to YouTube API service account credentials
	ServiceAccountPath = "service-account.json"

	// DefaultAPIPort is the default port for the HTTP API server
	DefaultAPIPort = ":8081"
)

func main() {
	// Command-line flags
	batchMode := flag.Bool("batch", false, "Run in batch mode (process files from input/ directory)")
	apiPort := flag.String("port", DefaultAPIPort, "API server port (e.g., :8081)")
	flag.Parse()

	log.Println("🎬 Video Creation Service - Starting...")

	// Initialize video processor
	proc, err := services.NewVideoProcessor(ServiceAccountPath, config.BackgroundsDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialize processor: %v", err)
	}

	if *batchMode {
		// Batch mode: Process all files in input/ directory
		log.Println("📁 Running in BATCH mode")
		if err := proc.ProcessFromDirectory(config.InputDir); err != nil {
			log.Fatalf("❌ Batch processing failed: %v", err)
		}
		os.Exit(0)
	}

	// API mode: Start HTTP server
	log.Println("🌐 Running in API mode")

	apiServer := api.NewServer(proc)
	mux := apiServer.SetupRoutes()

	log.Printf("🚀 API Server listening on %s", *apiPort)
	log.Println("📌 Endpoints:")
	log.Println("   POST /api/process-video  - Process video from JSON")
	log.Println("   GET  /health             - Health check")

	if err := http.ListenAndServe(*apiPort, mux); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}
