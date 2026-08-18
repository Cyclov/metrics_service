package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/Cyclov/metrics_service/internal/config"
	"github.com/Cyclov/metrics_service/internal/logger"
	"github.com/Cyclov/metrics_service/internal/server"
	"go.uber.org/zap"
)

func main() {
	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	cfg, err := config.ServerConfig()
	if err != nil {
		logger.Log.Fatal("failed to parse server config", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		logger.Log.Fatal("server can't start", zap.Error(err))
	}
}
