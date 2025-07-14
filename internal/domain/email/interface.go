package email

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, email *Email) error
	GetByID(ctx context.Context, id uuid.UUID) (*Email, error)
	Update(ctx context.Context, email *Email) error
	GetPendingEmails(ctx context.Context, limit int) ([]*Email, error)
}

type QueueMessage struct {
	EmailID uuid.UUID        `json:"email_id"`
	Type    EmailType        `json:"type"`
	Data    WelcomeEmailData `json:"data"`
}

type Publisher interface {
	PublishWelcomeEmail(ctx context.Context, data WelcomeEmailData) error
	Close() error
}

type Consumer interface {
	StartConsuming(ctx context.Context, handler MessageHandler) error
	Close() error
}

type MessageHandler func(ctx context.Context, message QueueMessage) error

type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

type EmailService interface {
	SendEmail(ctx context.Context, email *Email) error
	SendEmailDev(ctx context.Context, email *Email) error
	SendEmailAuto(ctx context.Context, email *Email) error
}
