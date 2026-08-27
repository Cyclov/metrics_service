package main

import (
	"log"
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
	sender := agent.NewSender("http://"+cfg.SrvAdr, nil)

	if err := agent.Run(
		collector,
		sender,
		time.Duration(cfg.PollInterval)*time.Second,
		time.Duration(cfg.ReportInterval)*time.Second,
	); err != nil {
		log.Fatal(err)
	}
}
