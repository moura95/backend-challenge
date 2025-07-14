package main

import (
	"context"
	"log"
	"sync"
	"time"

	emailUC "github.com/moura95/backend-challenge/internal/application/usecases/email"
	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/database/postgres"
	"github.com/moura95/backend-challenge/internal/infra/email/smtp"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
	"github.com/moura95/backend-challenge/internal/infra/repository/adapters"
	"github.com/moura95/backend-challenge/internal/interfaces/http/handlers"
	"go.uber.org/zap"
)

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
	smtpService := smtp.NewSMTPServiceDev(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

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
