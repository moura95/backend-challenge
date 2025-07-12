package main

import (
	"log"

	"github.com/moura95/backend-challenge/internal/infra/config"
	"github.com/moura95/backend-challenge/internal/infra/http/gin"
	"go.uber.org/zap"
)

func main() {
	// Configs
	loadConfig, _ := config.LoadConfig(".")

	// instance Db
	//conn, err := postgres.ConnectPostgres()
	////store := conn.DB()
	////if err != nil {
	////	fmt.Println("Failed to Connected Database")
	////	panic(err)
	////}
	log.Print("connection is repository establish")

	// Zap Logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	// Run Gin
	gin.RunGinServer(loadConfig, sugar)
}
