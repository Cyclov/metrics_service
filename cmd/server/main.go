package main

import (
	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/server"
	"go.uber.org/zap"
)

func main() {

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	logger.Log.Fatal("server can't start", zap.Error(server.Run(config.ServerConfig())))
}
