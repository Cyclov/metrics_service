package main

import (
	"log"
	"time"

	"github.com/Cyclov/metrics_service/internal/agent"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
	serverAddress  = "http://localhost:8080"
) //Потом перенесу в конфиг

func main() {
	collector := agent.NewCollector()
	sender := agent.NewSender(serverAddress, nil)
	if err := agent.Run(collector, sender, pollInterval, reportInterval); err != nil {
		log.Fatal(err)
	}
}
