package main

import (
	"log"

	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/database/postgres"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
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
	rabbitConn := setupRabbitMQ(loadConfig, sugar)
	if rabbitConn != nil {
		defer rabbitConn.Close()
		sugar.Info("RabbitMQ connection established")
	}

	// Run HTTP server - Passa SÓ a conexão RabbitMQ
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
