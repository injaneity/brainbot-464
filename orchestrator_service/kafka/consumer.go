package kafka

import (
	"context"
	"log"
	"orchestrator/state"
	"orchestrator/types"

	sharedKafka "brainbot/shared/kafka"
)

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers      []string
	Topic        string
	GroupID      string
	StateManager *state.Manager
}

// orchestratorMessageHandler implements the MessageHandler interface for orchestrator
type orchestratorMessageHandler struct {
	stateManager *state.Manager
}

// NewConsumer creates a new Kafka consumer using the shared consumer implementation
func NewConsumer(config ConsumerConfig) (*sharedKafka.Consumer, error) {
	handler := &sharedKafka.TypedMessageHandler[types.WebhookPayload]{
		Validate: func(msg *types.WebhookPayload) bool {
			if msg.UUID == "" {
				log.Printf("❌ Message missing UUID, skipping")
				return false
			}
			return true
		},
		Process: func(ctx context.Context, msg *types.WebhookPayload) error {
			// Note: We process both "success" and "failure" statuses here
			// to update the orchestrator state accordingly.
			config.StateManager.SetWebhookPayload(msg)
			log.Printf("✅ State updated from Kafka message for UUID: %s (Status: %s)", msg.UUID, msg.Status)
			return nil
		},
		AlwaysMark: true, // Always mark messages, even validation failures
	}

	return sharedKafka.NewConsumer(sharedKafka.ConsumerConfig{
		Brokers: config.Brokers,
		Topic:   config.Topic,
		GroupID: config.GroupID,
		Handler: handler,
	})
}
