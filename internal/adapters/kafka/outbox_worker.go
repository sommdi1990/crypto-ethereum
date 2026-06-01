package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/rs/zerolog"

	"github.com/soroush/eth-exchange-lab/internal/application/ports"
)

// Producer publishes messages to Kafka/Redpanda.
type Producer struct {
	writer *kafkago.Writer
}

// NewProducer creates a Kafka producer.
func NewProducer(brokers string) *Producer {
	return &Producer{
		writer: &kafkago.Writer{
			Addr:     kafkago.TCP(brokers),
			Balancer: &kafkago.LeastBytes{},
		},
	}
}

// Publish sends a message to a topic.
func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
	})
}

// Close closes the producer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// OutboxWorker publishes unpublished outbox events.
type OutboxWorker struct {
	log      zerolog.Logger
	outbox   ports.OutboxRepository
	producer *Producer
	interval time.Duration
}

// NewOutboxWorker creates an outbox publisher worker.
func NewOutboxWorker(log zerolog.Logger, outbox ports.OutboxRepository, producer *Producer, interval time.Duration) *OutboxWorker {
	return &OutboxWorker{log: log, outbox: outbox, producer: producer, interval: interval}
}

// Run polls outbox and publishes events.
func (w *OutboxWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.publishBatch(ctx); err != nil {
				w.log.Error().Err(err).Msg("outbox publish")
			}
		}
	}
}

func (w *OutboxWorker) publishBatch(ctx context.Context) error {
	events, err := w.outbox.FetchUnpublished(ctx, 50)
	if err != nil {
		return err
	}
	for _, e := range events {
		payload, _ := json.Marshal(e.Payload)
		topic := mapEventTypeToTopic(e.EventType)
		if err := w.producer.Publish(ctx, topic, e.AggregateID.String(), payload); err != nil {
			return fmt.Errorf("publish %s: %w", e.EventType, err)
		}
		if err := w.outbox.MarkPublished(ctx, e.ID); err != nil {
			return err
		}
	}
	return nil
}

func mapEventTypeToTopic(eventType string) string {
	switch eventType {
	case "exchange.deposit.detected":
		return "exchange.deposit.detected"
	case "exchange.deposit.credited":
		return "exchange.deposit.credited"
	case "exchange.order.placed":
		return "exchange.order.placed"
	case "exchange.order.cancelled":
		return "exchange.order.cancelled"
	case "exchange.trade.executed":
		return "exchange.trade.executed"
	case "exchange.withdrawal.requested":
		return "exchange.withdrawal.requested"
	case "exchange.withdrawal.broadcasted":
		return "exchange.withdrawal.broadcasted"
	case "exchange.blockchain.reorg":
		return "exchange.blockchain.reorg"
	default:
		return "exchange.events"
	}
}
