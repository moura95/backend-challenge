package email

import (
	"fmt"
	"regexp"
)

type EmailValidator struct{}

func NewEmailValidator() *EmailValidator {
	return &EmailValidator{}
}

func (v *EmailValidator) ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func (v *EmailValidator) ValidateSubject(subject string) error {
	if subject == "" {
		return fmt.Errorf("email subject is required")
	}

	if len(subject) > 255 {
		return fmt.Errorf("email subject must be less than 255 characters")
	}

	return nil
}

func (v *EmailValidator) ValidateBody(body string) error {
	if body == "" {
		return fmt.Errorf("email body is required")
	}

	if len(body) > 65535 { // 64KB limit
		return fmt.Errorf("email body is too large")
	}

	return nil
}

func (v *EmailValidator) ValidateType(emailType EmailType) error {
	switch emailType {
	case EmailTypeWelcome:
		return nil
	default:
		return fmt.Errorf("invalid email type: %s", emailType)
	}
}

func (v *EmailValidator) ValidateEmailEntity(email *Email) error {
	if err := v.ValidateEmail(email.To); err != nil {
		return err
	}

	if err := v.ValidateSubject(email.Subject); err != nil {
		return err
	}

	if err := v.ValidateBody(email.Body); err != nil {
		return err
	}

	if err := v.ValidateType(email.Type); err != nil {
		return err
	}

	if email.MaxAttempts <= 0 || email.MaxAttempts > 10 {
		return fmt.Errorf("max attempts must be between 1 and 10")
	}

	return nil
}

func (v *EmailValidator) ValidateWelcomeEmailData(data WelcomeEmailData) error {
	if data.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	if data.UserName == "" {
		return fmt.Errorf("user name is required")
	}

	if err := v.ValidateEmail(data.UserEmail); err != nil {
		return fmt.Errorf("user email validation failed: %w", err)
	}

	return nil
}
