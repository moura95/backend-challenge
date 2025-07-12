package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	userUC "github.com/moura95/backend-challenge/internal/application/usecases/user"
	userDomain "github.com/moura95/backend-challenge/internal/domain/user"
	"github.com/moura95/backend-challenge/internal/interfaces/http/middlewares"
	"github.com/moura95/backend-challenge/pkg/ginx"
)

type UserHandler struct {
	getUserProfileUseCase *userUC.GetUserProfileUseCase
	updateUserUseCase     *userUC.UpdateUserUseCase
	deleteUserUseCase     *userUC.DeleteUserUseCase
	listUsersUseCase      *userUC.ListUsersUseCase
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

func NewUserHandler(
	getUserProfileUC *userUC.GetUserProfileUseCase,
	updateUserUC *userUC.UpdateUserUseCase,
	deleteUserUC *userUC.DeleteUserUseCase,
	listUsersUC *userUC.ListUsersUseCase,
) *UserHandler {
	return &UserHandler{
		getUserProfileUseCase: getUserProfileUC,
		updateUserUseCase:     updateUserUC,
		deleteUserUseCase:     deleteUserUC,
		listUsersUseCase:      listUsersUC,
	}
}

// @Summary Get user profile
// @Description Get current user profile information
// @Tags user
// @Security BearerAuth
// @Produce json
// @Success 200 {object} ginx.Response{data=userDomain.UserResponse}
// @Failure 401 {object} ginx.Response
// @Failure 404 {object} ginx.Response
// @Router /account/me [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := middlewares.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("handler: get profile failed: user not authenticated"))
		return
	}

	foundUser, err := h.getUserProfileUseCase.Execute(c.Request.Context(), userID)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: get profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(foundUser.ToResponse()))
}

// @Summary Update user profile
// @Description Update current user profile information
// @Tags user
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body UpdateUserRequest true "Update user request"
// @Success 200 {object} ginx.Response{data=userDomain.UserResponse}
// @Failure 400 {object} ginx.Response
// @Failure 401 {object} ginx.Response
// @Failure 409 {object} ginx.Response
// @Router /account/me [put]
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

	updatedUser, err := h.updateUserUseCase.Execute(c.Request.Context(), userID, updateReq)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: update profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusOK, ginx.SuccessResponse(updatedUser.ToResponse()))
}

// @Summary Delete user profile
// @Description Delete current user account
// @Tags user
// @Security BearerAuth
// @Success 204 "No content"
// @Failure 401 {object} ginx.Response
// @Failure 404 {object} ginx.Response
// @Router /account/me [delete]
func (h *UserHandler) DeleteProfile(c *gin.Context) {
	userID, exists := middlewares.GetUserIDFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("handler: delete profile failed: user not authenticated"))
		return
	}

	err := h.deleteUserUseCase.Execute(c.Request.Context(), userID)
	if err != nil {
		statusCode := getStatusCodeFromError(err)
		c.JSON(statusCode, ginx.ErrorResponse(fmt.Sprintf("handler: delete profile failed: %v", err)))
		return
	}

	c.JSON(http.StatusNoContent, ginx.SuccessResponse(nil))
}

// @Summary List users
// @Description Get paginated list of users with optional search
// @Tags user
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param search query string false "Search by name or email"
// @Produce json
// @Success 200 {object} ginx.Response{data=ListUsersResponse}
// @Failure 400 {object} ginx.Response
// @Failure 401 {object} ginx.Response
// @Router /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	search := c.Query("search")

	req := userUC.ListUsersRequest{
		Page:     page,
		PageSize: pageSize,
		Search:   search,
	}

	result, err := h.listUsersUseCase.Execute(c.Request.Context(), req)
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
