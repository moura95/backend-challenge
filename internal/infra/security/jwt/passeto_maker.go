package jwt

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/o1egl/paseto"
	"go/token"
	"golang.org/x/crypto/chacha20poly1305"
	"time"
)

type PasetoMaker struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewPasetoMaker(symmetricKey string) (Maker, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: must be exactly %d characters", chacha20poly1305.KeySize)
	}

	maker := &PasetoMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}
	return maker, nil
}

func (maker *PasetoMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, *token.Payload, error) {
	payload, err := token.NewPayload(userID, duration)
	if err != nil {
		return "", payload, err
	}

	tokenStr, err := maker.paseto.Encrypt(maker.symmetricKey, payload, nil)
	return tokenStr, payload, err
}

func (maker *PasetoMaker) VerifyToken(tokenStr string) (*token.Payload, error) {
	payload := &token.Payload{}

	err := maker.paseto.Decrypt(tokenStr, maker.symmetricKey, payload, nil)
	if err != nil {
		return nil, token.ErrInvalidToken
	}

	err = payload.Valid()
	if err != nil {
		return nil, err
	}

	return payload, nil
}
