package gin

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	userUC "github.com/moura95/backend-challenge/internal/application/usecases/user"
	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
	"github.com/moura95/backend-challenge/internal/interfaces/http/handlers"
	"github.com/moura95/backend-challenge/internal/interfaces/http/middlewares"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag/example/basic/docs"
	"go.uber.org/zap"
)

type Server struct {
	router *gin.Engine
	config *config.Config
	logger *zap.SugaredLogger
}

// @title           Backend Challenge
// @version         1.0
// @description     Backend Challenge API
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:6000
// @BasePath  /api
func NewServer(cfg config.Config, db *sqlx.DB, log *zap.SugaredLogger) *Server {
	server := &Server{
		config: &cfg,
		logger: log,
	}

	router := gin.Default()
	docs.SwaggerInfo.BasePath = "/api"

	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddAllowHeaders("Content-Type")
	router.Use(cors.New(corsConfig))

	createRoutes(cfg, db, router, log)

	server.router = router
	return server
}

func createRoutes(cfg config.Config, db *sqlx.DB, router *gin.Engine, log *zap.SugaredLogger) {
	// Initialize repositories
	repositories := adapters.NewRepositories(db)

	// Initialize JWT token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012") // 32 chars for demo
	if err != nil {
		log.Fatalf("Failed to create token maker: %v", err)
	}

	// Auth use cases
	signUpUC := authUC.NewSignUpUseCase(
		repositories.User,
		repositories.Email,
		tokenMaker,
		nil, // email publisher será configurado depois
	)
	signInUC := authUC.NewSignInUseCase(repositories.User, tokenMaker)
	verifyTokenUC := authUC.NewVerifyTokenUseCase(repositories.User, tokenMaker)

	// User use cases
	getUserProfileUC := userUC.NewGetUserProfileUseCase(repositories.User)
	updateUserUC := userUC.NewUpdateUserUseCase(repositories.User)
	deleteUserUC := userUC.NewDeleteUserUseCase(repositories.User)
	listUsersUC := userUC.NewListUsersUseCase(repositories.User)

	// Email use cases
	//sendWelcomeEmailUC := emailUC.NewSendWelcomeEmailUseCase(
	//	repositories.Email,
	//	nil, // email publisher será configurado depois
	//)
	//processEmailQueueUC := emailUC.NewProcessEmailQueueUseCase(
	//	repositories.Email,
	//	nil, // email sender será configurado depois
	//)

	authHandler := handlers.NewAuthHandler(signUpUC, signInUC, verifyTokenUC)
	userHandler := handlers.NewUserHandler(getUserProfileUC, updateUserUC, deleteUserUC, listUsersUC)

	// Public routes
	api := router.Group("/api")
	{
		authRoutes := api.Group("/auth")
		{
			authRoutes.POST("/signup", authHandler.SignUp)
			authRoutes.POST("/signin", authHandler.SignIn)
		}
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(middlewares.AuthMiddleware(verifyTokenUC))
	{
		account := protected.Group("/account")
		{
			account.GET("/me", userHandler.GetProfile)
			account.PUT("/me", userHandler.UpdateProfile)
			account.DELETE("/me", userHandler.DeleteProfile)
		}

		// Bonus routes
		protected.GET("/users", userHandler.ListUsers)
	}
}

func (s *Server) Start(address string) error {
	s.logger.Infof("Starting server on %s", address)
	return s.router.Run(address)
}

func RunGinServer(cfg config.Config, db *sqlx.DB, log *zap.SugaredLogger) {
	server := NewServer(cfg, db, log)
	if err := server.Start(cfg.HTTPServerAddress); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
