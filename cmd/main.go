package main

import (
	"context"
	"log"
	"sync"
	"time"

	emailUC "github.com/moura95/backend-challenge/internal/application/usecases/email"
	"github.com/moura95/backend-challenge/internal/domain/email"
	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/database/postgres"
	"github.com/moura95/backend-challenge/internal/infra/email/smtp"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/interfaces/http/handlers"
	"go.uber.org/zap"

	_ "github.com/moura95/backend-challenge/docs"
)

// @title           Backend Challenge API
// @version         1.0
// @description     API RESTful completa para gestão de usuários com Clean Architecture + DDD
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token. Example: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

func main() {
	// Load configuration
	loadConfig, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// Initialize database connection
	conn, err := postgres.ConnectPostgres()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close()
	db := conn.DB()
	sugar.Info("Database connection established")

	// Initialize repositories
	repositories := adapters.NewRepositories(db)

	// Initialize RabbitMQ connection
	rabbitConn := setupRabbitMQ(loadConfig, sugar)
	if rabbitConn != nil {
		defer rabbitConn.Close()
		sugar.Info("RabbitMQ connection established")
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start email consumer if RabbitMQ is available
	if rabbitConn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startEmailConsumer(ctx, loadConfig, repositories, rabbitConn, sugar)
		}()
	}

	// Log Swagger information
	sugar.Info("🚀 Starting Backend Challenge API")
	sugar.Info("📚 Swagger UI: http://localhost:8080/swagger/index.html")
	sugar.Info("🔐 Use Bearer tokens for authentication")

	// Run HTTP server
	gin.RunGinServer(loadConfig, db, sugar, rabbitConn)
}

func setupRabbitMQ(cfg config.Config, logger *zap.SugaredLogger) *rabbitmq.Connection {
	connectionConfig := rabbitmq.ConnectionConfig{
		URL: cfg.RabbitMQURL,
	}

	rabbitConn, err := rabbitmq.NewConnection(connectionConfig)
	if err != nil {
		logger.Warnf("Failed to setup RabbitMQ (continuing without messaging): %v", err)
		return nil // App continua sem messaging
	}

	logger.Info("RabbitMQ connection configured successfully")
	return rabbitConn
}

func startEmailConsumer(
	ctx context.Context,
	cfg config.Config,
	repositories *adapters.Repositories,
	rabbit *rabbitmq.Connection,
	logger *zap.SugaredLogger,
) {
	// Setup SMTP service
	smtpService := smtp.NewSMTPService(
		email.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: "",
			Password: "",
			From:     cfg.SMTPFrom,
		})

	// Setup email processing use case
	processEmailUC := emailUC.NewProcessEmailQueueUseCase(
		repositories.Email,
		smtpService,
	)
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			processEmailUC.ProcessPendingEmails(ctx, 50)
		}
	}()

	// Setup email consumer handler
	emailHandler := handlers.NewEmailConsumerHandler(processEmailUC)

	// Start consuming emails
	err := rabbit.StartEmailConsumer(
		ctx,
		emailHandler.HandleEmailMessage,
		"email_notifications",
	)

	if err != nil {
		logger.Errorf("Email consumer stopped with error: %v", err)
	} else {
		logger.Info("Email consumer stopped gracefully")
	}
}
