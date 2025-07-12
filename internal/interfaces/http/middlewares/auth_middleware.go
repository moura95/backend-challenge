package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	"github.com/moura95/backend-challenge/pkg/ginx"
)

const (
	authorizationHeaderKey  = "authorization"
	authorizationTypeBearer = "bearer"
	userIDKey               = "user_id"
)

func AuthMiddleware(verifyTokenUseCase *authUC.VerifyTokenUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorizationHeader := c.GetHeader(authorizationHeaderKey)

		if len(authorizationHeader) == 0 {
			c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("middleware: authorization header not provided"))
			c.Abort()
			return
		}

		fields := strings.Fields(authorizationHeader)
		if len(fields) < 2 {
			c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("middleware: invalid authorization header format"))
			c.Abort()
			return
		}

		authorizationType := strings.ToLower(fields[0])
		if authorizationType != authorizationTypeBearer {
			c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("middleware: unsupported authorization type"))
			c.Abort()
			return
		}

		accessToken := fields[1]

		user, err := verifyTokenUseCase.Execute(c.Request.Context(), accessToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, ginx.ErrorResponse("middleware: invalid or expired token"))
			c.Abort()
			return
		}

		c.Set(userIDKey, user.ID.String())
		c.Next()
	}
}

func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get(userIDKey)
	if !exists {
		return "", false
	}

	userIDStr, ok := userID.(string)
	if !ok {
		return "", false
	}

	return userIDStr, true
}
