package main

import (
	"context"
	"flag"
	"fmt"
	"orchestrator/api"
	"orchestrator/client"
	"orchestrator/kafka"
	"orchestrator/state"
	"orchestrator/workflow"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	_ = godotenv.Load()

	// Parse command-line flags
	port := flag.String("port", "8081", "HTTP API port")
	webhookPort := flag.String("webhook-port", "9999", "Webhook server port")
	cronSchedule := flag.String("cron", "*/5 * * * *", "Cron schedule for automated runs (default: every 5 minutes)")
	apiURL := flag.String("api-url", "", "Ingestion service API URL (overrides API_URL env var)")
	flag.Parse()

	// Determine ingestion service URL
	ingestionURL := *apiURL
	if ingestionURL == "" {
		ingestionURL = os.Getenv("API_URL")
		if ingestionURL == "" {
			ingestionURL = "http://ingestion-service:8080"
		}
	}

	// Create ingestion service client
	ingestionClient := client.NewIngestionClient(ingestionURL)

	// Create state manager
	stateManager := state.NewManager(*webhookPort, ingestionClient)

	// Create workflow runner
	workflowRunner := workflow.NewRunner(stateManager)

	// Kafka configuration
	kafkaBrokers := []string{"kafka:9092"} // Default
	if envBrokers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS"); envBrokers != "" {
		kafkaBrokers = []string{envBrokers}
	}
	kafkaTopic := "video-processing-requests"
	kafkaGroupID := "orchestrator-consumer-group"

	// Create Kafka consumer
	consumerConfig := kafka.ConsumerConfig{
		Brokers:      kafkaBrokers,
		Topic:        kafkaTopic,
		GroupID:      kafkaGroupID,
		StateManager: stateManager,
	}

	kafkaConsumer, err := kafka.NewConsumer(consumerConfig)
	if err != nil {
		fmt.Printf("Failed to create Kafka consumer: %v\n", err)
	} else {
		// Start consumer
		ctx := context.Background()
		if err := kafkaConsumer.Start(ctx); err != nil {
			fmt.Printf("Failed to start Kafka consumer: %v\n", err)
		}
	}

	// Create and start API server
	apiServer := api.NewServer(stateManager, workflowRunner, *port)

	if err := apiServer.Start(); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Start cron job
	if err := apiServer.StartCron(*cronSchedule); err != nil {
		fmt.Printf("Failed to start cron: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🤖 Orchestrator Service\n")
	fmt.Printf("   API:            http://0.0.0.0:%s\n", *port)
	fmt.Printf("   Webhook:        http://0.0.0.0:%s/webhook\n", *webhookPort)
	fmt.Printf("   Cron Schedule:  %s\n", *cronSchedule)
	fmt.Printf("   Ingestion API:  %s\n", ingestionURL)
	fmt.Println("\nPress Ctrl+C to shutdown")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := apiServer.Shutdown(ctx); err != nil {
		fmt.Printf("Shutdown error: %v\n", err)
		os.Exit(1)
	}

	if kafkaConsumer != nil {
		if err := kafkaConsumer.Close(); err != nil {
			fmt.Printf("Kafka consumer close error: %v\n", err)
		}
	}

	fmt.Println("Server stopped")
}
