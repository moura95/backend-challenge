package email

import (
	"time"

	"github.com/google/uuid"
)

type EmailType string

const (
	EmailTypeWelcome EmailType = "welcome"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

type Email struct {
	ID          uuid.UUID  `json:"id"`
	To          string     `json:"to"`
	Subject     string     `json:"subject"`
	Body        string     `json:"body"`
	Type        EmailType  `json:"type"`
	Status      Status     `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
}

type WelcomeEmailData struct {
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}

func NewWelcomeEmail(data WelcomeEmailData) (*Email, error) {
	validator := NewEmailValidator()

	if err := validator.ValidateWelcomeEmailData(data); err != nil {
		return nil, err
	}

	email := &Email{
		ID:          uuid.New(),
		To:          data.UserEmail,
		Subject:     "Welcome to Backend Challenge!",
		Body:        generateWelcomeEmailBody(data.UserName),
		Type:        EmailTypeWelcome,
		Status:      StatusPending,
		Attempts:    0,
		MaxAttempts: 3,
		CreatedAt:   time.Now(),
	}

	if err := validator.ValidateEmailEntity(email); err != nil {
		return nil, err
	}

	return email, nil
}

func (e *Email) MarkAsSent() {
	e.Status = StatusSent
	now := time.Now()
	e.SentAt = &now
}

func (e *Email) MarkAsFailed(errorMsg string) {
	e.Attempts++
	e.ErrorMsg = errorMsg

	if e.Attempts >= e.MaxAttempts {
		e.Status = StatusFailed
	} else {
		e.Status = StatusPending
	}
}

func (e *Email) CanRetry() bool {
	return e.Status == StatusPending && e.Attempts < e.MaxAttempts
}

func generateWelcomeEmailBody(userName string) string {
	return `
<!DOCTYPE html>
<html>
<head>
    <title>Welcome!</title>
</head>
<body>
    <h1>Welcome to Backend Challenge, ` + userName + `!</h1>
    <p>Thank you for signing up! We're excited to have you on board.</p>
    <p>Best regards,<br>The Backend Challenge Team</p>
</body>
</html>
`
}
