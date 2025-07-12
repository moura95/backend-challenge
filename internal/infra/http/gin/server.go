package gin

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/moura95/backend-challenge/internal/infra/config"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/swag/example/basic/docs"

	"go.uber.org/zap"
)

type Server struct {
	//store  *repository.Querier
	router *gin.Engine
	config *config.Config
	logger *zap.SugaredLogger
}

// @title           Backend Challenge
// @version         1.0
// @description     desc
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1
// func NewServer(cfg config.Config, store repository.Querier, log *zap.SugaredLogger) *Server {
func NewServer(cfg config.Config, log *zap.SugaredLogger) *Server {

	server := &Server{
		//store:  &store,
		config: &cfg,
		logger: log,
	}
	var router *gin.Engine

	router = gin.Default()

	docs.SwaggerInfo.BasePath = ""

	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	//router.Use(middleware.RateLimitMiddleware())
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("redirect")
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddAllowHeaders("Content-Type")
	router.Use(cors.New(corsConfig))

	//createRoutesV1(&store, server.config, router, log)
	createRoutesV1(server.config, router, log)

	server.router = router
	return server
}

// func createRoutesV1(store *repository.Querier, cfg *config.Config, router *gin.Engine, log *zap.SugaredLogger) {
func createRoutesV1(cfg *config.Config, router *gin.Engine, log *zap.SugaredLogger) {
	//userService := service.NewUserService(*store, *cfg, log)

	//userHandler := handler.NewUserHandler(userService, cfg, log)

	//apiV1 := router.Group("/api/v1")
	//{
	//	packages := apiV1.Group("/packages")
	//	{
	//		packages.GET("/:id", userHandler.GetByID)
	//		packages.POST("", userHandler.Create)
	//		packages.DELETE("/:id", userHandler.Delete)
	//	}
	//
	//}
}

func (s *Server) Start(address string) error {
	return s.router.Run(address)
}

// func RunGinServer(cfg config.Config, store repository.Querier, log *zap.SugaredLogger) {
func RunGinServer(cfg config.Config, log *zap.SugaredLogger) {
	//server := NewServer(cfg, store, log)
	server := NewServer(cfg, log)

	_ = server.Start(cfg.HTTPServerAddress)
}
