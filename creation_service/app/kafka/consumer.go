package kafka

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/services"
	sharedKafka "brainbot/shared/kafka"
)

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers   []string
	Topic     string
	GroupID   string
	Processor *services.VideoProcessor
}

// NewConsumer creates a new Kafka consumer using the shared consumer implementation
func NewConsumer(config ConsumerConfig) (*sharedKafka.Consumer, error) {
	handler := &sharedKafka.TypedMessageHandler[app.VideoInput]{
		Validate: func(msg *app.VideoInput) bool {
			// Validate status
			if msg.Status != "success" {
				log.Printf("⚠️  Skipping message with status: %s", msg.Status)
				return false
			}

			// Validate UUID
			if msg.UUID == "" {
				log.Printf("❌ Message missing UUID, skipping")
				return false
			}

			return true
		},
		Process: func(ctx context.Context, msg *app.VideoInput) error {
			log.Printf("🎬 Processing video: UUID=%s", msg.UUID)

			// Process video
			if err := config.Processor.ProcessVideoInput(*msg, false); err != nil {
				log.Printf("❌ Failed to process video %s: %v", msg.UUID, err)
				return err // Return error to prevent marking (allow retry)
			}

			log.Printf("✅ Successfully processed video: UUID=%s", msg.UUID)
			return nil
		},
		AlwaysMark: true, // Mark validation failures, but not processing failures
	}

	return sharedKafka.NewConsumer(sharedKafka.ConsumerConfig{
		Brokers: config.Brokers,
		Topic:   config.Topic,
		GroupID: config.GroupID,
		Handler: handler,
	})
}

// StartConsumerWithGracefulShutdown starts the Kafka consumer with graceful shutdown handling
func StartConsumerWithGracefulShutdown(config ConsumerConfig) error {
	consumer, err := NewConsumer(config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer
	if err := consumer.Start(ctx); err != nil {
		return err
	}

	// Wait for interrupt signal
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigterm:
		log.Println("Received termination signal")
	case <-ctx.Done():
		log.Println("Context canceled")
	}

	cancel()

	// Give some time for in-flight processing to complete
	time.Sleep(2 * time.Second)

	return consumer.Close()
}

// GetKafkaBrokers parses Kafka broker list from environment variable
func GetKafkaBrokers() []string {
	brokers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	if brokers == "" {
		brokers = "localhost:9093"
	}
	return strings.Split(brokers, ",")
}

// GetKafkaTopic returns the Kafka topic name from environment variable
func GetKafkaTopic() string {
	topic := os.Getenv("KAFKA_TOPIC_VIDEO_REQUESTS")
	if topic == "" {
		topic = "video-processing-requests"
	}
	return topic
}

// GetKafkaGroupID returns the Kafka consumer group ID
func GetKafkaGroupID() string {
	groupID := os.Getenv("KAFKA_CONSUMER_GROUP_ID")
	if groupID == "" {
		groupID = "creation-service-consumer-group"
	}
	return groupID
}
