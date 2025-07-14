package rabbitmq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/streadway/amqp"
)

func (c *Connection) PublishWelcomeEmailMessage(message email.QueueMessage) error {
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
