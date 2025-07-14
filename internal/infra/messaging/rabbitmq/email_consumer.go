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
	messages, err := c.channel.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	log.Printf("%s consumer started", queueName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("%s consumer stopped", queueName)
			return nil
		case msg := <-messages:
			var queueMessage email.QueueMessage

			// Parse + Process + Ack
			if json.Unmarshal(msg.Body, &queueMessage) == nil {
				if handler(ctx, queueMessage) == nil {
					msg.Ack(false)
					log.Printf("Email processed: %s", queueMessage.Data.UserEmail)
				} else {
					msg.Reject(true) // Retry
				}
			} else {
				msg.Reject(false) // Não retry se não conseguir fazer parse
			}
		}
	}
}
