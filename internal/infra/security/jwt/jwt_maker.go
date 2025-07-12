package jwt

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/o1egl/paseto"
)

type JWTMaker struct {
	secretKey string
}

func NewJWTMaker(secretKey string) (Maker, error) {
	if len(secretKey) < 32 {
		return nil, fmt.Errorf("invalid key size: must be at least 32 characters")
	}

	return &JWTMaker{secretKey}, nil
}

func (maker *JWTMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, Payload, error) {
	payload, err := NewPayload(userID, duration)
	if err != nil {
		return "", *payload, err
	}

	pasetoMaker := paseto.NewV2()
	token, err := pasetoMaker.Encrypt([]byte(maker.secretKey[:32]), payload, nil)
	return token, *payload, err
}

func (maker *JWTMaker) VerifyToken(token string) (*Payload, error) {
	payload := &Payload{}

	pasetoMaker := paseto.NewV2()
	err := pasetoMaker.Decrypt(token, []byte(maker.secretKey[:32]), payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	err = payload.Valid()
	if err != nil {
		return nil, err
	}

	return payload, nil
}
