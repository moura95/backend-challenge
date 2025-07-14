package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/moura95/backend-challenge/internal/domain/email"
)

func (c *Connection) StartEmailConsumer(ctx context.Context, handler email.MessageHandler, queueName string) error {
	if !c.IsConnected() {
		return fmt.Errorf("RabbitMQ not connected")
	}

	// Consumir mensagens
	messages, err := c.channel.Consume(
		queueName,
		"",    // consumer name
		false, // auto-ack = false
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	log.Printf("%s consumer started", queueName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("%s consumer stopped", queueName)
			return nil

		case msg, ok := <-messages:
			if !ok {
				log.Printf("Messages channel closed for %s", queueName)
				return fmt.Errorf("messages channel closed")
			}

			var queueMessage email.QueueMessage

			// 1. Parse da mensagem
			if err := json.Unmarshal(msg.Body, &queueMessage); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				msg.Reject(false) // Mensagem malformada, descarta
				continue
			}

			// 2. Processar mensagem
			if err := handler(ctx, queueMessage); err != nil {
				log.Printf("Failed to process email message: %v", err)
				msg.Ack(false)
			} else {
				log.Printf("Email processed successfully for user %s", queueMessage.Data.UserEmail)
				msg.Ack(false)
			}
		}
	}
}
