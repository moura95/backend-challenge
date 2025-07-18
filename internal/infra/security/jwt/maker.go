package jwt

import (
	"time"

	"github.com/google/uuid"
)

type Maker interface {
	CreateToken(userID uuid.UUID, duration time.Duration) (string, Payload, error)
	VerifyToken(token string) (*Payload, error)
}
