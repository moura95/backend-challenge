// internal/infra/messaging/rabbitmq/publisher_functions.go
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/streadway/amqp"
)

func (c *Connection) PublishWelcomeEmail(ctx context.Context, data email.WelcomeEmailData) error {
	message := email.QueueMessage{
		EmailID: uuid.New(),
		Type:    email.EmailTypeWelcome,
		Data:    data,
	}

	return c.publishToEmailQueue(message)
}

func (c *Connection) publishToEmailQueue(message interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("rabbitmq: connection not available")
	}

	// Marshal message
	messageBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to marshal message: %w", err)
	}

	// Create AMQP message
	amqpMessage := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         messageBody,
		MessageId:    uuid.New().String(),
	}

	// Publish ONLY to email queue
	err = c.channel.Publish(
		"",                    // exchange (empty for direct queue)
		"email_notifications", // routing key = queue name
		false,                 // mandatory
		false,                 // immediate
		amqpMessage,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to publish to email queue: %w", err)
	}

	fmt.Printf("Published welcome email to queue\n")
	return nil
}

func (c *Connection) publishToQueue(queueName string, message interface{}) error {
	if !c.IsConnected() {
		return fmt.Errorf("rabbitmq: connection not available")
	}

	// Marshal message
	messageBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to marshal message: %w", err)
	}

	// Create AMQP message
	amqpMessage := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         messageBody,
		MessageId:    uuid.New().String(),
	}

	// Publish to queue
	err = c.channel.Publish(
		"",        // exchange (empty for direct queue)
		queueName, // routing key = queue name
		false,     // mandatory
		false,     // immediate
		amqpMessage,
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: failed to publish to %s: %w", queueName, err)
	}

	fmt.Printf("Published message to queue: %s\n", queueName)
	return nil
}
