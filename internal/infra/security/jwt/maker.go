package jwt

import (
	"github.com/google/uuid"
	"go/token"
	"time"
)

type Maker interface {
	CreateToken(userID uuid.UUID, duration time.Duration) (string, *token.Payload, error)
	VerifyToken(token string) (*token.Payload, error)
}
