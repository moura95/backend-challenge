package rabbitmq

import (
	"fmt"
	"log"
	"time"

	"github.com/streadway/amqp"
)

type Connection struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	url     string
}

type ConnectionConfig struct {
	URL string
}

func NewConnection(config ConnectionConfig) (*Connection, error) {
	conn := &Connection{
		url: config.URL,
	}

	err := conn.connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	err = conn.setupQueue("email_notifications")
	if err != nil {
		return nil, fmt.Errorf("failed to setup email queue: %w", err)
	}

	return conn, nil
}

func (c *Connection) connect() error {
	var err error

	// Retry connection with backoff
	for i := 0; i < 5; i++ {
		c.conn, err = amqp.Dial(c.url)
		if err == nil {
			break
		}

		log.Printf("Failed to connect to RabbitMQ (attempt %d/5): %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect after 5 attempts: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	log.Println("Successfully connected to RabbitMQ")
	return nil
}

func (c *Connection) setupQueue(queueName string) error {
	_, err := c.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			"x-message-ttl": 3600000, // 1 hour TTL
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare email queue: %w", err)
	}

	log.Printf("Email queue setup completed")
	return nil
}

func (c *Connection) Channel() *amqp.Channel {
	return c.channel
}

func (c *Connection) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Connection) IsConnected() bool {
	return c.conn != nil && !c.conn.IsClosed()
}
