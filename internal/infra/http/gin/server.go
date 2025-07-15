package gin

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	authUC "github.com/moura95/backend-challenge/internal/application/usecases/auth"
	userUC "github.com/moura95/backend-challenge/internal/application/usecases/user"
	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/infra/security/jwt"
	"github.com/moura95/backend-challenge/internal/interfaces/http/handlers"
	"github.com/moura95/backend-challenge/internal/interfaces/http/middlewares"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	_ "github.com/moura95/backend-challenge/docs"
)

type Server struct {
	router *gin.Engine
	config *config.Config
	logger *zap.SugaredLogger
	rabbit *rabbitmq.Connection
}

func NewServer(cfg config.Config, db *sqlx.DB, log *zap.SugaredLogger, rabbit *rabbitmq.Connection) *Server {
	server := &Server{
		config: &cfg,
		logger: log,
		rabbit: rabbit,
	}

	router := gin.Default()

	// Health check endpoint
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// 🚨 SWAGGER CONFIGURATION - URL específica para o doc.json
	url := ginSwagger.URL("http://localhost:8080/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler, url))

	// CORS configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddAllowHeaders("Content-Type")
	router.Use(cors.New(corsConfig))

	// Setup routes
	createRoutes(cfg, db, router, log, rabbit)

	server.router = router
	return server
}

func createRoutes(cfg config.Config, db *sqlx.DB, router *gin.Engine, log *zap.SugaredLogger, rabbit *rabbitmq.Connection) {
	// Initialize repositories
	repositories := adapters.NewRepositories(db)

	// Initialize JWT token maker
	tokenMaker, err := jwt.NewPasetoMaker("12345678901234567890123456789012") // 32 chars for demo
	if err != nil {
		log.Fatalf("Failed to create token maker: %v", err)
	}

	// Initialize use cases
	signUpUC := authUC.NewSignUpUseCase(
		repositories.User,
		repositories.Email,
		tokenMaker,
		rabbit,
	)
	signInUC := authUC.NewSignInUseCase(repositories.User, tokenMaker)
	verifyTokenUC := authUC.NewVerifyTokenUseCase(repositories.User, tokenMaker)

	getUserProfileUC := userUC.NewGetUserProfileUseCase(repositories.User)
	updateUserUC := userUC.NewUpdateUserUseCase(repositories.User)
	deleteUserUC := userUC.NewDeleteUserUseCase(repositories.User)
	listUsersUC := userUC.NewListUsersUseCase(repositories.User)

	// Initialize handlers
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

		protected.GET("/users", userHandler.ListUsers)
	}

	log.Info("Routes configured successfully")
}

func (s *Server) Start(address string) error {
	s.logger.Infof("Starting server on %s", address)
	s.logger.Infof("Swagger UI available at: http://localhost:8080/swagger/index.html")
	return s.router.Run(address)
}

func RunGinServer(cfg config.Config, db *sqlx.DB, log *zap.SugaredLogger, rabbit *rabbitmq.Connection) {
	server := NewServer(cfg, db, log, rabbit)

	if err := server.Start(cfg.HTTPServerAddress); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
