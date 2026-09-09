package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cyclov/metrics_service/internal/agent"
	"github.com/Cyclov/metrics_service/internal/config"
)

func main() {
	cfg, err := config.AgentConfig()
	if err != nil {
		log.Fatal(err)
	}

	collector := agent.NewCollector()
	sender := agent.NewSender("http://"+cfg.SrvAdr, nil, cfg.Key)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := agent.Run(
		ctx,
		collector,
		sender,
		time.Duration(cfg.PollInterval)*time.Second,
		time.Duration(cfg.ReportInterval)*time.Second,
	); err != nil {
		log.Fatal(err)
	}
}
