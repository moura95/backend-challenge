package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/moura95/backend-challenge/internal/application/services/user"
	userUC "github.com/moura95/backend-challenge/internal/application/usecases/user"
	userDomain "github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/interfaces/http/middlewares"
	"github.com/moura95/backend-challenge/pkg/ginx"
)

type UserHandler struct {
	userService *user.UserService
}

type UpdateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ListUsersResponse struct {
	Users []*userDomain.UserResponse `json:"users"`
	Total int                        `json:"total"`
	Page  int                        `json:"page"`
}

func NewUserHandler(userService *user.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := middlewares.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("handler: get profile failed: user not authenticated"))
		return
	}

	foundUser, err := h.userService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: get profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(foundUser.ToResponse()))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, exists := middlewares.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("handler: update profile failed: user not authenticated"))
		return
	}

	var req UpdateUserRequest
	if err := ginx.ParseJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, ginx.ErrorResponse("handler: update profile failed: invalid request format"))
		return
	}

	updateReq := userUC.UpdateUserRequest{
		Name:  req.Name,
		Email: req.Email,
	}

	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), userID, updateReq)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: update profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(updatedUser.ToResponse()))
}

func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID, exists := middlewares.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("handler: delete profile failed: user not authenticated"))
		return
	}

	err := h.userService.DeleteProfile(c.Request.Context(), userID)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: delete profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusNoContent, ginx.SuccessResponse(nil))
}

func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	search := c.Query("search")

	req := userUC.ListUsersRequest{
		Page:     page,
		PageSize: pageSize,
		Search:   search,
	}

	result, err := h.userService.ListUsers(c.Request.Context(), req)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: list users failed: %v", err)))
		return
	}

	userResponses := make([]*userDomain.UserResponse, len(result.Users))
	for i, u := range result.Users {
		response := u.ToResponse()
		userResponses[i] = &response
	}

	response := ListUsersResponse{
		Users: userResponses,
		Total: result.Total,
		Page:  result.Page,
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(response))
}
