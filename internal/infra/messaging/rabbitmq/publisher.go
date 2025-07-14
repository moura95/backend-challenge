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

type Publisher struct {
	connection *Connection
	queueName  string
}

func NewPublisher(connection *Connection, queueName string) *Publisher {
	return &Publisher{
		connection: connection,
		queueName:  queueName,
	}
}

func (p *Publisher) PublishWelcomeEmail(ctx context.Context, data email.WelcomeEmailData) error {
	if !p.connection.IsConnected() {
		return fmt.Errorf("publisher: RabbitMQ connection is not available")
	}

	// Create queue message
	message := email.QueueMessage{
		EmailID: uuid.New(),
		Type:    email.EmailTypeWelcome,
		Data:    data,
	}

	// Marshal message to JSON
	messageBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("publisher: failed to marshal message: %w", err)
	}

	// Prepare AMQP message
	amqpMessage := amqp.Publishing{
		DeliveryMode: amqp.Persistent, // Mensagem persistente
		Timestamp:    time.Now(),
		ContentType:  "application/json",
		Body:         messageBody,
		MessageId:    message.EmailID.String(),
		Type:         string(message.Type),
	}

	err = p.connection.Channel().Publish(
		"",          // exchange = ""
		p.queueName, // queue
		false,       // mandatory
		false,       // immediate
		amqpMessage,
	)
	if err != nil {
		return fmt.Errorf("publisher: failed to publish message: %w", err)
	}

	fmt.Printf("Published welcome email message for user %s (email: %s)\n",
		data.UserID, data.UserEmail)

	return nil
}

func (p *Publisher) Close() error {
	return nil
}

func (p *Publisher) GetQueueName() string {
	return p.queueName
}
