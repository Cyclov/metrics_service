package main

import (
	"log"

	"github.com/Cyclov/metrics_service/internal/agent"
	"github.com/Cyclov/metrics_service/internal/config"
)

func main() {
	cfg := config.AgentConfig()
	collector := agent.NewCollector()
	sender := agent.NewSender("http://"+cfg.SrvAdr, nil)

	if err := agent.Run(collector, sender, cfg.PollInterval, cfg.ReportInterval); err != nil {
		log.Fatal(err)
	}
}
