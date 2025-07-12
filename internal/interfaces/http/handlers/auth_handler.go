package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authSvc "github.com/moura95/backend-challenge/internal/application/services/auth"
	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	"github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/pkg/ginx"
)

type AuthHandler struct {
	authService *authSvc.AuthService
}

type AuthResponse struct {
	User  user.UserResponse `json:"user"`
	Token string            `json:"token"`
}

func NewAuthHandler(authService *authSvc.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) SignUp(c *gin.Context) {
	var req authUC.SignUpRequest

	if err := ginx.ParseJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.ErrorResponse("handler: signup failed: invalid request format"))
		return
	}

	result, err := h.authService.SignUp(c.Request.Context(), req)
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

func (h *AuthHandler) SignIn(c *gin.Context) {
	var req authUC.SignInRequest

	if err := ginx.ParseJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.ErrorResponse("handler: signin failed: invalid request format"))
		return
	}

	result, err := h.authService.SignIn(c.Request.Context(), req)
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
