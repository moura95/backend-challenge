package main

import (
	"log"

	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/database/postgres"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	loadConfig, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database connection
	conn, err := postgres.ConnectPostgres()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close()

	db := conn.DB()
	log.Print("Database connection established")

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	// Run HTTP server
	gin.RunGinServer(loadConfig, db, sugar)
}
