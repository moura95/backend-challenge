package user

import (
	"time"

	"github.com/google/uuid"
	"github.com/moura95/backend-challenge/internal/infra/security/crypto"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Never expose password in JSON
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewUser(name, email, password string) (*User, error) {
	validator := NewUserValidator()

	// Create user instance
	user := &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Validate user data
	if err := validator.ValidateUser(user); err != nil {
		return nil, err
	}

	// Validate password strength
	if err := validator.ValidatePassword(password); err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user.Password = hashedPassword

	return user, nil
}

func (u *User) UpdateUser(name, email string) error {
	validator := NewUserValidator()

	if name != "" {
		if err := validator.ValidateName(name); err != nil {
			return err
		}
		u.Name = name
	}

	if email != "" {
		if err := validator.ValidateEmail(email); err != nil {
			return err
		}
		u.Email = email
	}

	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) CheckPassword(password string) error {
	return crypto.CheckPassword(password, u.Password)
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}
