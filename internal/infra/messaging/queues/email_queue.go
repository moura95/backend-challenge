package queues

import (
	"context"
	"fmt"
	"log"

	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
)

type EmailQueue struct {
	publisher *rabbitmq.Publisher
	consumer  *rabbitmq.Consumer
}

func NewEmailQueue(connection *rabbitmq.Connection, queueName string) *EmailQueue {
	publisher := rabbitmq.NewPublisher(connection, queueName)
	consumer := rabbitmq.NewConsumer(connection, queueName)

	return &EmailQueue{
		publisher: publisher,
		consumer:  consumer,
	}
}

func (eq *EmailQueue) PublishWelcomeEmail(ctx context.Context, data email.WelcomeEmailData) error {
	err := eq.publisher.PublishWelcomeEmail(ctx, data)
	if err != nil {
		return fmt.Errorf("email queue: failed to publish welcome email: %w", err)
	}

	log.Printf("Welcome email queued for user %s (%s)", data.UserName, data.UserEmail)
	return nil
}

func (eq *EmailQueue) StartConsuming(ctx context.Context, handler email.MessageHandler) error {
	err := eq.consumer.StartConsuming(ctx, handler)
	if err != nil {
		return fmt.Errorf("email queue: failed to start consuming: %w", err)
	}

	log.Println("Email queue consumer started")
	return nil
}

func (eq *EmailQueue) Close() error {
	var publisherErr, consumerErr error

	if eq.publisher != nil {
		publisherErr = eq.publisher.Close()
	}

	if eq.consumer != nil {
		consumerErr = eq.consumer.Close()
	}

	if publisherErr != nil {
		return fmt.Errorf("failed to close publisher: %w", publisherErr)
	}

	if consumerErr != nil {
		return fmt.Errorf("failed to close consumer: %w", consumerErr)
	}

	return nil
}

func (eq *EmailQueue) GetPublisher() *rabbitmq.Publisher {
	return eq.publisher
}

func (eq *EmailQueue) GetConsumer() *rabbitmq.Consumer {
	return eq.consumer
}
