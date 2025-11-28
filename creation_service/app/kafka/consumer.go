package kafka

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"brainbot/creation_service/app"
	"brainbot/creation_service/app/services"

	"github.com/IBM/sarama"
)

// Consumer handles Kafka message consumption
type Consumer struct {
	consumer  sarama.ConsumerGroup
	processor *services.VideoProcessor
	topic     string
	groupID   string
	ready     chan bool
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers   []string
	Topic     string
	GroupID   string
	Processor *services.VideoProcessor
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(config ConsumerConfig) (*Consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V3_6_0_0
	saramaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(config.Brokers, config.GroupID, saramaConfig)
	if err != nil {
		return nil, err
	}

	consumer := &Consumer{
		consumer:  client,
		processor: config.Processor,
		topic:     config.Topic,
		groupID:   config.GroupID,
		ready:     make(chan bool),
	}

	return consumer, nil
}

// Start begins consuming messages from Kafka
func (c *Consumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		consumer:  c,
		processor: c.processor,
		ready:     c.ready,
	}

	go func() {
		for {
			if err := c.consumer.Consume(ctx, []string{c.topic}, handler); err != nil {
				if err == context.Canceled {
					log.Println("Kafka consumer context canceled")
					return
				}
				log.Printf("Error from Kafka consumer: %v", err)
			}

			if ctx.Err() != nil {
				return
			}
			handler.ready = make(chan bool)
		}
	}()

	<-c.ready
	log.Printf("✅ Kafka consumer started (group: %s, topic: %s)", c.groupID, c.topic)

	// Handle errors
	go func() {
		for err := range c.consumer.Errors() {
			log.Printf("❌ Kafka consumer error: %v", err)
		}
	}()

	return nil
}

// Close gracefully shuts down the consumer
func (c *Consumer) Close() error {
	log.Println("Closing Kafka consumer...")
	return c.consumer.Close()
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer  *Consumer
	processor *services.VideoProcessor
	ready     chan bool
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			log.Printf("📥 Received Kafka message: partition=%d, offset=%d, key=%s",
				message.Partition, message.Offset, string(message.Key))

			// Parse message
			var videoInput app.VideoInput
			if err := json.Unmarshal(message.Value, &videoInput); err != nil {
				log.Printf("❌ Failed to unmarshal message: %v", err)
				session.MarkMessage(message, "")
				continue
			}

			// Validate
			if videoInput.Status != "success" {
				log.Printf("⚠️  Skipping message with status: %s", videoInput.Status)
				session.MarkMessage(message, "")
				continue
			}

			if videoInput.UUID == "" {
				log.Printf("❌ Message missing UUID, skipping")
				session.MarkMessage(message, "")
				continue
			}

			log.Printf("🎬 Processing video: UUID=%s", videoInput.UUID)

			// Process video
			if err := h.processor.ProcessVideoInput(videoInput, false); err != nil {
				log.Printf("❌ Failed to process video %s: %v", videoInput.UUID, err)
				// Don't mark message - will retry
				continue
			}

			log.Printf("✅ Successfully processed video: UUID=%s", videoInput.UUID)

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
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
