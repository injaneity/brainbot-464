package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"brainbot/creation_service/app/api"
	"brainbot/creation_service/app/config"
	"brainbot/creation_service/app/kafka"
	"brainbot/creation_service/app/services"
)

const (
	// DefaultAPIPort is the default port for the HTTP API server
	DefaultAPIPort = ":8081"
)

func main() {
	// Command-line flags
	batchMode := flag.Bool("batch", false, "Run in batch mode (process files from input/ directory)")
	kafkaMode := flag.Bool("kafka", false, "Run in Kafka consumer mode (consume from Kafka topic)")
	apiPort := flag.String("port", DefaultAPIPort, "API server port (e.g., :8081)")
	flag.Parse()

	log.Println("🎬 Video Creation Service - Starting...")

	// Initialize video processor
	proc, err := services.NewVideoProcessor(config.BackgroundsDir)
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

	if *kafkaMode {
		// Kafka mode: Consume from Kafka topic
		log.Println("📨 Running in KAFKA consumer mode")

		kafkaConfig := kafka.ConsumerConfig{
			Brokers:   kafka.GetKafkaBrokers(),
			Topic:     kafka.GetKafkaTopic(),
			GroupID:   kafka.GetKafkaGroupID(),
			Processor: proc,
		}

		log.Printf("🔗 Kafka Brokers: %v", kafkaConfig.Brokers)
		log.Printf("📋 Topic: %s", kafkaConfig.Topic)
		log.Printf("👥 Consumer Group: %s", kafkaConfig.GroupID)

		if err := kafka.StartConsumerWithGracefulShutdown(kafkaConfig); err != nil {
			log.Fatalf("❌ Kafka consumer failed: %v", err)
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
