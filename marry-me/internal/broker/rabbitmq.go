package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"

	"marry-me/internal/config"
	"marry-me/internal/event"
)

// RabbitMQ implements the Broker interface using RabbitMQ
type RabbitMQ struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	cfg      *config.Config
	exchange string
	mu       sync.RWMutex
	closed   bool
}

// NewRabbitMQ creates a new RabbitMQ broker instance
func NewRabbitMQ(cfg *config.Config) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		cfg:      cfg,
		exchange: cfg.ExchangeName,
	}

	if err := rmq.connect(); err != nil {
		return nil, err
	}

	return rmq, nil
}

// connect establishes connection to RabbitMQ with retry logic
func (r *RabbitMQ) connect() error {
	var err error
	maxRetries := 5
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		r.conn, err = amqp.Dial(r.cfg.RabbitMQURL)
		if err == nil {
			break
		}

		log.Warn().
			Err(err).
			Int("attempt", i+1).
			Int("max_retries", maxRetries).
			Msg("Failed to connect to RabbitMQ, retrying...")

		time.Sleep(retryDelay)
		retryDelay *= 2
	}

	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ after %d attempts: %w", maxRetries, err)
	}

	r.channel, err = r.conn.Channel()
	if err != nil {
		r.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare exchange
	err = r.channel.ExchangeDeclare(
		r.exchange, // name
		"topic",    // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		r.channel.Close()
		r.conn.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	log.Info().
		Str("exchange", r.exchange).
		Msg("Connected to RabbitMQ")

	// Set up connection close notification
	go r.handleConnectionClose()

	return nil
}

// handleConnectionClose handles connection recovery
func (r *RabbitMQ) handleConnectionClose() {
	notifyClose := make(chan *amqp.Error)
	r.conn.NotifyClose(notifyClose)

	err := <-notifyClose
	if err != nil {
		r.mu.Lock()
		if !r.closed {
			log.Error().
				Err(err).
				Msg("RabbitMQ connection lost, attempting reconnect...")

			// Attempt reconnect
			if reconnectErr := r.connect(); reconnectErr != nil {
				log.Error().
					Err(reconnectErr).
					Msg("Failed to reconnect to RabbitMQ")
			}
		}
		r.mu.Unlock()
	}
}

// DeclareQueue creates a queue if it doesn't exist
func (r *RabbitMQ) DeclareQueue(ctx context.Context, name string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, err := r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", name, err)
	}

	log.Debug().
		Str("queue", name).
		Msg("Queue declared")

	return nil
}

// BindQueue binds a queue to an exchange with a routing key
func (r *RabbitMQ) BindQueue(ctx context.Context, queue, exchange, routingKey string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	err := r.channel.QueueBind(
		queue,      // queue name
		routingKey, // routing key
		exchange,   // exchange
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s: %w", queue, err)
	}

	log.Debug().
		Str("queue", queue).
		Str("exchange", exchange).
		Str("routing_key", routingKey).
		Msg("Queue bound to exchange")

	return nil
}

// Publish sends an event to the specified routing key
func (r *RabbitMQ) Publish(ctx context.Context, routingKey string, e *event.Event) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return fmt.Errorf("broker is closed")
	}

	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		r.exchange,  // exchange
		routingKey,  // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Debug().
		Str("event_id", e.ID).
		Str("routing_key", routingKey).
		Msg("Event published")

	return nil
}

// Subscribe starts consuming events from a queue
func (r *RabbitMQ) Subscribe(ctx context.Context, queue string, handler EventHandler) error {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return fmt.Errorf("broker is closed")
	}

	msgs, err := r.channel.Consume(
		queue,                // queue
		"",                   // consumer tag
		false,                // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	r.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Info().
		Str("queue", queue).
		Msg("Started consuming from queue")

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Info().
					Str("queue", queue).
					Msg("Stopping consumer")
				return

			case msg, ok := <-msgs:
				if !ok {
					log.Warn().
						Str("queue", queue).
						Msg("Consumer channel closed")
					return
				}

				var e event.Event
				if err := json.Unmarshal(msg.Body, &e); err != nil {
					log.Error().
						Err(err).
						Str("queue", queue).
						Msg("Failed to unmarshal event")
					msg.Nack(false, false)
					continue
				}

				if err := handler(ctx, &e); err != nil {
					log.Error().
						Err(err).
						Str("event_id", e.ID).
						Str("queue", queue).
						Msg("Handler error")
					msg.Nack(false, true) // Requeue on handler error
					continue
				}

				msg.Ack(false)
			}
		}
	}()

	return nil
}

// Close closes the broker connection
func (r *RabbitMQ) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	var errs []error

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close channel: %w", err))
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	log.Info().Msg("RabbitMQ connection closed")
	return nil
}

// SetupQueues creates all required queues and bindings
func (r *RabbitMQ) SetupQueues(ctx context.Context) error {
	queues := []string{
		r.cfg.IncomingQueue,
		r.cfg.ValidatedQueue,
		r.cfg.ResultsQueue,
	}

	// Add team queues
	for _, team := range event.AllTeams() {
		queues = append(queues, config.GetTeamQueue(string(team)))
	}

	for _, queue := range queues {
		if err := r.DeclareQueue(ctx, queue); err != nil {
			return err
		}

		// Bind queue to exchange with queue name as routing key
		if err := r.BindQueue(ctx, queue, r.exchange, queue); err != nil {
			return err
		}
	}

	log.Info().
		Int("count", len(queues)).
		Msg("All queues set up")

	return nil
}
