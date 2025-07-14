package main

import (
	"log"

	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/database/postgres"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
	"github.com/moura95/backend-challenge/internal/infra/messaging/queues"
	"github.com/moura95/backend-challenge/internal/infra/messaging/rabbitmq"
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

	// Initialize RabbitMQ connection
	rabbitConn, emailQueue, err := setupRabbitMQ(loadConfig, sugar)
	if err != nil {
		sugar.Warnf("Failed to setup RabbitMQ (continuing without queue): %v", err)
		// Continue without RabbitMQ for development
	} else {
		defer rabbitConn.Close()
		defer emailQueue.Close()
		sugar.Info("RabbitMQ connection established")
	}

	// Run HTTP server
	gin.RunGinServer(loadConfig, db, sugar, emailQueue)
}

func setupRabbitMQ(cfg config.Config, logger *zap.SugaredLogger) (*rabbitmq.Connection, *queues.EmailQueue, error) {
	// Setup RabbitMQ connection
	connectionConfig := rabbitmq.ConnectionConfig{
		URL: cfg.RabbitMQURL,
	}

	rabbitConn, err := rabbitmq.NewConnection(connectionConfig)
	if err != nil {
		return nil, nil, err
	}

	// Setup email queue
	emailQueue := queues.NewEmailQueue(
		rabbitConn,
		"email_notifications",
	)

	logger.Info("RabbitMQ exchange and queues configured")
	return rabbitConn, emailQueue, nil
}
