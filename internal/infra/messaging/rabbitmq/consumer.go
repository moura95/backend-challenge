package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/streadway/amqp"
)

type Consumer struct {
	connection *Connection
	queueName  string
	handler    email.MessageHandler
}

func NewConsumer(connection *Connection, queueName string) *Consumer {
	return &Consumer{
		connection: connection,
		queueName:  queueName,
	}
}

func (c *Consumer) StartConsuming(ctx context.Context, handler email.MessageHandler) error {
	if !c.connection.IsConnected() {
		return fmt.Errorf("consumer: RabbitMQ connection is not available")
	}

	c.handler = handler

	// Configure QoS - process one message at a time
	err := c.connection.Channel().Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("consumer: failed to set QoS: %w", err)
	}

	// Start consuming messages
	messages, err := c.connection.Channel().Consume(
		c.queueName, // queue
		"",          // consumer tag (empty = auto-generated)
		false,       // auto-ack (we'll ack manually)
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("consumer: failed to register consumer: %w", err)
	}

	log.Printf("Consumer started for queue: %s", c.queueName)

	// Process messages in a goroutine
	go c.processMessages(ctx, messages)

	return nil
}

func (c *Consumer) processMessages(ctx context.Context, messages <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Consumer context cancelled, stopping message processing")
			return
		case msg, ok := <-messages:
			if !ok {
				log.Println("Message channel closed, stopping consumer")
				return
			}
			c.handleMessage(ctx, msg)
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, delivery amqp.Delivery) {
	startTime := time.Now()
	messageID := delivery.MessageId

	log.Printf("Processing message ID: %s, Type: %s", messageID, delivery.Type)

	// Parse message
	var queueMessage email.QueueMessage
	err := json.Unmarshal(delivery.Body, &queueMessage)
	if err != nil {
		log.Printf("Failed to unmarshal message ID %s: %v", messageID, err)
		c.rejectMessage(delivery, false) // Don't requeue malformed messages
		return
	}

	// Validate message
	if err := c.validateMessage(queueMessage); err != nil {
		log.Printf("Invalid message ID %s: %v", messageID, err)
		c.rejectMessage(delivery, false) // Don't requeue invalid messages
		return
	}

	// Process message with timeout
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err = c.handler(processCtx, queueMessage)
	if err != nil {
		log.Printf("Failed to process message ID %s: %v", messageID, err)
		c.handleProcessingError(delivery, err)
		return
	}

	// Acknowledge successful processing
	err = delivery.Ack(false)
	if err != nil {
		log.Printf("Failed to ack message ID %s: %v", messageID, err)
	} else {
		duration := time.Since(startTime)
		log.Printf("Successfully processed message ID %s in %v", messageID, duration)
	}
}

func (c *Consumer) validateMessage(msg email.QueueMessage) error {
	if msg.EmailID.String() == "" {
		return fmt.Errorf("invalid email ID")
	}

	if msg.Type == "" {
		return fmt.Errorf("message type is required")
	}

	// Validate based on message type
	switch msg.Type {
	case email.EmailTypeWelcome:
		if msg.Data.UserID == "" {
			return fmt.Errorf("user ID is required for welcome email")
		}
		if msg.Data.UserEmail == "" {
			return fmt.Errorf("user email is required for welcome email")
		}
		if msg.Data.UserName == "" {
			return fmt.Errorf("user name is required for welcome email")
		}
	default:
		return fmt.Errorf("unsupported message type: %s", msg.Type)
	}

	return nil
}

func (c *Consumer) handleProcessingError(delivery amqp.Delivery, err error) {
	// Get retry count from headers
	retryCount := c.getRetryCount(delivery.Headers)
	maxRetries := 3

	if retryCount >= maxRetries {
		log.Printf("Message ID %s exceeded max retries (%d), sending to DLQ", delivery.MessageId, maxRetries)
		c.rejectMessage(delivery, false) // Don't requeue after max retries
		return
	}

	// Increment retry count and requeue
	log.Printf("Requeuing message ID %s (retry %d/%d)", delivery.MessageId, retryCount+1, maxRetries)

	c.rejectMessage(delivery, true) // Requeue for retry
}

func (c *Consumer) getRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	if count, ok := headers["retry_count"]; ok {
		if intCount, ok := count.(int); ok {
			return intCount
		}
		if int32Count, ok := count.(int32); ok {
			return int(int32Count)
		}
	}

	return 0
}

func (c *Consumer) rejectMessage(delivery amqp.Delivery, requeue bool) {
	err := delivery.Reject(requeue)
	if err != nil {
		log.Printf("Failed to reject message ID %s: %v", delivery.MessageId, err)
	}
}

func (c *Consumer) Close() error {
	// Consumer doesn't own the connection, so no cleanup needed
	log.Println("Consumer closed")
	return nil
}

func (c *Consumer) GetQueueName() string {
	return c.queueName
}
