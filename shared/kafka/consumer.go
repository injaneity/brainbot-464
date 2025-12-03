package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
)

// MessageHandler defines the interface for handling consumed messages
// Each service implements this to provide custom message processing logic
type MessageHandler interface {
	// HandleMessage processes a Kafka message and returns whether to mark it as processed
	// If error is returned, the message will not be marked (allowing retry)
	// If shouldMark is false, the message will not be marked (allowing retry)
	HandleMessage(ctx context.Context, message []byte) (shouldMark bool, err error)
}

// Consumer handles Kafka message consumption with pluggable message handling
type Consumer struct {
	consumer sarama.ConsumerGroup
	handler  MessageHandler
	topic    string
	groupID  string
	ready    chan bool
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
	Handler MessageHandler
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
		consumer: client,
		handler:  config.Handler,
		topic:    config.Topic,
		groupID:  config.GroupID,
		ready:    make(chan bool),
	}

	return consumer, nil
}

// Start begins consuming messages from Kafka
func (c *Consumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		consumer:       c,
		messageHandler: c.handler,
		ready:          c.ready,
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
	consumer       *Consumer
	messageHandler MessageHandler
	ready          chan bool
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages()
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			log.Printf("📥 Received Kafka message: partition=%d, offset=%d, key=%s",
				message.Partition, message.Offset, string(message.Key))

			// Delegate to custom handler
			shouldMark, err := h.messageHandler.HandleMessage(session.Context(), message.Value)
			if err != nil {
				log.Printf("❌ Failed to handle message: %v", err)
			}

			// Mark message if handler indicates success
			if shouldMark {
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}

// TypedMessageHandler is a generic helper that handles type conversion
// T is the message type (e.g., WebhookPayload, VideoInput)
type TypedMessageHandler[T any] struct {
	// Validate checks if the message should be processed
	Validate func(msg *T) bool
	// Process handles the actual message processing
	Process func(ctx context.Context, msg *T) error
	// AlwaysMark determines if messages should be marked even on validation failure
	AlwaysMark bool
}

// HandleMessage implements MessageHandler interface
func (h *TypedMessageHandler[T]) HandleMessage(ctx context.Context, message []byte) (bool, error) {
	var msg T
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("❌ Failed to unmarshal message: %v", err)
		return h.AlwaysMark, nil // Mark to skip invalid messages
	}

	// Validate message
	if h.Validate != nil && !h.Validate(&msg) {
		// Validation failed - mark if AlwaysMark is true
		return h.AlwaysMark, nil
	}

	// Process message
	if err := h.Process(ctx, &msg); err != nil {
		return false, err // Don't mark - allow retry
	}

	return true, nil // Success - mark message
}
