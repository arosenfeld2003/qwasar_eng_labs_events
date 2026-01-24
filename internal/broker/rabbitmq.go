package broker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig defines connection and channel settings.
type RabbitMQConfig struct {
	URL            string
	ConnectionName string
	Heartbeat      time.Duration
	Locale         string
	ChannelMax     int
	FrameSize      int
	PrefetchCount  int
	PrefetchSize   int
}

// RabbitMQ implements Broker using RabbitMQ.
type RabbitMQ struct {
	cfg    RabbitMQConfig
	conn   *amqp.Connection
	mu     sync.Mutex
	closed bool
}

// NewRabbitMQ establishes a new RabbitMQ connection.
func NewRabbitMQ(ctx context.Context, cfg RabbitMQConfig) (*RabbitMQ, error) {
	if cfg.URL == "" {
		return nil, errors.New("rabbitmq url is required")
	}

	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 10 * time.Second
	}
	if cfg.Locale == "" {
		cfg.Locale = "en_US"
	}

	config := amqp.Config{
		Heartbeat: cfg.Heartbeat,
		Locale:    cfg.Locale,
		Properties: amqp.Table{
			"connection_name": cfg.ConnectionName,
		},
		Dial: func(network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
	if cfg.ChannelMax > 0 {
		config.ChannelMax = uint16(cfg.ChannelMax)
	}
	if cfg.FrameSize > 0 {
		config.FrameSize = cfg.FrameSize
	}

	conn, err := amqp.DialConfig(cfg.URL, config)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	return &RabbitMQ{cfg: cfg, conn: conn}, nil
}

// Close shuts down the broker connection.
func (b *RabbitMQ) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	conn := b.conn
	b.mu.Unlock()

	if conn == nil {
		return nil
	}
	if err := conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
		return fmt.Errorf("close connection: %w", err)
	}
	return nil
}

// DeclareExchange declares an exchange.
func (b *RabbitMQ) DeclareExchange(ctx context.Context, name string, opts ExchangeOptions) error {
	ch, err := b.openChannel()
	if err != nil {
		return err
	}
	defer closeChannel(ch)

	kind := opts.Kind
	if kind == "" {
		kind = "direct"
	}

	if err := ch.ExchangeDeclare(
		name,
		kind,
		opts.Durable,
		opts.AutoDelete,
		opts.Internal,
		opts.NoWait,
		toTable(opts.Arguments),
	); err != nil {
		return fmt.Errorf("declare exchange %q: %w", name, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// DeclareQueue declares a queue.
func (b *RabbitMQ) DeclareQueue(ctx context.Context, name string, opts QueueOptions) (QueueInfo, error) {
	ch, err := b.openChannel()
	if err != nil {
		return QueueInfo{}, err
	}
	defer closeChannel(ch)

	q, err := ch.QueueDeclare(
		name,
		opts.Durable,
		opts.AutoDelete,
		opts.Exclusive,
		opts.NoWait,
		toTable(opts.Arguments),
	)
	if err != nil {
		return QueueInfo{}, fmt.Errorf("declare queue %q: %w", name, err)
	}

	select {
	case <-ctx.Done():
		return QueueInfo{}, ctx.Err()
	default:
		return QueueInfo{Name: q.Name, Messages: q.Messages, Consumers: q.Consumers}, nil
	}
}

// BindQueue binds a queue to an exchange with a routing key.
func (b *RabbitMQ) BindQueue(ctx context.Context, queue, exchange, routingKey string, args map[string]interface{}) error {
	ch, err := b.openChannel()
	if err != nil {
		return err
	}
	defer closeChannel(ch)

	if err := ch.QueueBind(queue, routingKey, exchange, false, toTable(args)); err != nil {
		return fmt.Errorf("bind queue %q to %q: %w", queue, exchange, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Publish sends a message to the broker.
func (b *RabbitMQ) Publish(ctx context.Context, msg Message, opts PublishOptions) error {
	ch, err := b.openChannel()
	if err != nil {
		return err
	}
	defer closeChannel(ch)

	publishing := amqp.Publishing{
		Body:          msg.Body,
		ContentType:   msg.ContentType,
		Headers:       toTable(msg.Headers),
		Timestamp:     msg.Timestamp,
		CorrelationId: msg.CorrelationID,
		ReplyTo:       msg.ReplyTo,
		DeliveryMode:  msg.DeliveryMode,
	}
	if publishing.Timestamp.IsZero() {
		publishing.Timestamp = time.Now()
	}

	if err := ch.PublishWithContext(
		ctx,
		opts.Exchange,
		opts.RoutingKey,
		opts.Mandatory,
		opts.Immediate,
		publishing,
	); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}

// Subscribe registers a consumer and returns deliveries.
func (b *RabbitMQ) Subscribe(ctx context.Context, opts ConsumeOptions) (*Subscription, error) {
	ch, err := b.openChannel()
	if err != nil {
		return nil, err
	}

	if b.cfg.PrefetchCount > 0 || b.cfg.PrefetchSize > 0 {
		if err := ch.Qos(b.cfg.PrefetchCount, b.cfg.PrefetchSize, false); err != nil {
			closeChannel(ch)
			return nil, fmt.Errorf("set qos: %w", err)
		}
	}

	consumerTag := opts.Consumer
	if consumerTag == "" {
		consumerTag = fmt.Sprintf("consumer-%d", time.Now().UnixNano())
	}

	deliveries, err := ch.Consume(
		opts.Queue,
		consumerTag,
		opts.AutoAck,
		opts.Exclusive,
		opts.NoLocal,
		opts.NoWait,
		toTable(opts.Arguments),
	)
	if err != nil {
		closeChannel(ch)
		return nil, fmt.Errorf("consume from %q: %w", opts.Queue, err)
	}

	out := make(chan Delivery, 128)
	var cancelOnce sync.Once
	cancel := func() error {
		var cancelErr error
		cancelOnce.Do(func() {
			if err := ch.Cancel(consumerTag, false); err != nil && !errors.Is(err, amqp.ErrClosed) {
				cancelErr = fmt.Errorf("cancel consumer: %w", err)
			}
			if err := ch.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) && cancelErr == nil {
				cancelErr = fmt.Errorf("close channel: %w", err)
			}
		})
		return cancelErr
	}

	go func() {
		<-ctx.Done()
		_ = cancel()
	}()

	go func() {
		defer close(out)
		for d := range deliveries {
			out <- deliveryFromAMQP(d)
		}
	}()

	return &Subscription{Deliveries: out, Cancel: cancel}, nil
}

func (b *RabbitMQ) openChannel() (*amqp.Channel, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed || b.conn == nil || b.conn.IsClosed() {
		return nil, errors.New("rabbitmq connection is closed")
	}

	ch, err := b.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	return ch, nil
}

func closeChannel(ch *amqp.Channel) {
	if ch == nil {
		return
	}
	_ = ch.Close()
}

func toTable(values map[string]interface{}) amqp.Table {
	if len(values) == 0 {
		return nil
	}
	table := amqp.Table{}
	for key, value := range values {
		table[key] = value
	}
	return table
}

func fromTable(values amqp.Table) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func deliveryFromAMQP(d amqp.Delivery) Delivery {
	return Delivery{
		Message: Message{
			Body:          d.Body,
			ContentType:   d.ContentType,
			CorrelationID: d.CorrelationId,
			ReplyTo:       d.ReplyTo,
			Headers:       fromTable(d.Headers),
			Timestamp:     d.Timestamp,
			DeliveryMode:  d.DeliveryMode,
		},
		DeliveryTag: d.DeliveryTag,
		Redelivered: d.Redelivered,
		Exchange:    d.Exchange,
		RoutingKey:  d.RoutingKey,
		Ack: func(multiple bool) error {
			return d.Ack(multiple)
		},
		Nack: func(requeue bool) error {
			return d.Nack(false, requeue)
		},
		Reject: func(requeue bool) error {
			return d.Reject(requeue)
		},
	}
}
