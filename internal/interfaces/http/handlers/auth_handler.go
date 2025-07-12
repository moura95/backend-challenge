package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/pkg/ginx"
)

type AuthHandler struct {
	signUpUseCase      *authUC.SignUpUseCase
	signInUseCase      *authUC.SignInUseCase
	verifyTokenUseCase *authUC.VerifyTokenUseCase
}

type AuthResponse struct {
	User  user.UserResponse `json:"user"`
	Token string            `json:"token"`
}

func NewAuthHandler(
	signUpUC *authUC.SignUpUseCase,
	signInUC *authUC.SignInUseCase,
	verifyTokenUC *authUC.VerifyTokenUseCase,
) *AuthHandler {
	return &AuthHandler{
		signUpUseCase:      signUpUC,
		signInUseCase:      signInUC,
		verifyTokenUseCase: verifyTokenUC,
	}
}

// @Summary Sign up a new user
// @Description Create a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body authUC.SignUpRequest true "Sign up request"
// @Success 201 {object} ginx.Response{data=AuthResponse}
// @Failure 400 {object} ginx.Response
// @Failure 409 {object} ginx.Response
// @Router /auth/signup [post]
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req authUC.SignUpRequest

	if err := ginx.ParseJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.ErrorResponse("handler: signup failed: invalid request format"))
		return
	}

	result, err := h.signUpUseCase.Execute(c.Request.Context(), req)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: signup failed: %v", err)))
		return
	}

	response := AuthResponse{
		User:  result.User.ToResponse(),
		Token: result.Token,
	}

	c.JSON(http.StatusCreated, ginx.SuccessResponse(response))
}

// @Summary Sign in user
// @Description Authenticate user and return token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body authUC.SignInRequest true "Sign in request"
// @Success 200 {object} ginx.Response{data=AuthResponse}
// @Failure 400 {object} ginx.Response
// @Failure 401 {object} ginx.Response
// @Router /auth/signin [post]
func (h *AuthHandler) SignIn(c *gin.Context) {
	var req authUC.SignInRequest

	if err := ginx.ParseJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.ErrorResponse("handler: signin failed: invalid request format"))
		return
	}

	result, err := h.signInUseCase.Execute(c.Request.Context(), req)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: signin failed: %v", err)))
		return
	}

	response := AuthResponse{
		User:  result.User.ToResponse(),
		Token: result.Token,
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(response))
}

func (h *AuthHandler) VerifyToken(c *gin.Context, token string) (*user.User, error) {
	return h.verifyTokenUseCase.Execute(c.Request.Context(), token)
}

func getStatusCodeFromError(err error) int {
	errMsg := err.Error()

	if strings.Contains(errMsg, "email already exists") {
		return http.StatusConflict
	}

	if strings.Contains(errMsg, "invalid credentials") ||
		strings.Contains(errMsg, "user not found") ||
		strings.Contains(errMsg, "email is required") ||
		strings.Contains(errMsg, "password is required") {
		return http.StatusUnauthorized
	}

	if strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "required") ||
		strings.Contains(errMsg, "format") {
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}
