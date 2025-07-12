package user

import (
	"fmt"
	"regexp"

	"github.com/moura95/backend-challenge/internal/infra/security/crypto"
)

type UserValidator struct{}

func NewUserValidator() *UserValidator {
	return &UserValidator{}
}

func (v *UserValidator) ValidateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func (v *UserValidator) ValidateName(name string) error {
	if len(name) < 2 {
		return fmt.Errorf("name must be at least 2 characters long")
	}
	if len(name) > 100 {
		return fmt.Errorf("name must be less than 100 characters")
	}
	return nil
}

func (v *UserValidator) ValidatePassword(password string) error {
	return crypto.ValidatePasswordStrength(password)
}

func (v *UserValidator) ValidateUser(user *User) error {
	if err := v.ValidateName(user.Name); err != nil {
		return err
	}

	if err := v.ValidateEmail(user.Email); err != nil {
		return err
	}

	return nil
}
