package auth

import (
	"context"
	"fmt"

	"github.com/moura95/backend-challenge/internal/application/usecases/auth"
	"github.com/moura95/backend-challenge/internal/application/usecases/email"
	domainUser "github.com/moura95/backend-challenge/internal/domain/user"
)

type AuthService struct {
	signUpUseCase           *auth.SignUpUseCase
	signInUseCase           *auth.SignInUseCase
	verifyTokenUseCase      *auth.VerifyTokenUseCase
	sendWelcomeEmailUseCase *email.SendWelcomeEmailUseCase
}

func NewAuthService(
	signUpUC *auth.SignUpUseCase,
	signInUC *auth.SignInUseCase,
	verifyTokenUC *auth.VerifyTokenUseCase,
	sendWelcomeEmailUC *email.SendWelcomeEmailUseCase,
) *AuthService {
	return &AuthService{
		signUpUseCase:           signUpUC,
		signInUseCase:           signInUC,
		verifyTokenUseCase:      verifyTokenUC,
		sendWelcomeEmailUseCase: sendWelcomeEmailUC,
	}
}

func (s *AuthService) SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.SignUpResponse, error) {
	result, err := s.signUpUseCase.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("service: signup failed: %w", err)
	}

	emailReq := email.SendWelcomeEmailRequest{
		UserID:    result.User.ID.String(),
		UserName:  result.User.Name,
		UserEmail: result.User.Email,
	}

	err = s.sendWelcomeEmailUseCase.Execute(ctx, emailReq)
	if err != nil {
		fmt.Printf("Warning: failed to send welcome email: %v\n", err)
	}

	return result, nil
}

func (s *AuthService) SignIn(ctx context.Context, req auth.SignInRequest) (*auth.SignInResponse, error) {
	result, err := s.signInUseCase.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("service: signin failed: %w", err)
	}

	return result, nil
}

func (s *AuthService) VerifyToken(ctx context.Context, token string) (*domainUser.User, error) {
	result, err := s.verifyTokenUseCase.Execute(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("service: verify token failed: %w", err)
	}

	return result, nil
}
