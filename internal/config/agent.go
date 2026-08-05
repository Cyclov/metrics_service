package config

import (
	"flag"
	"time"
)

type AgentSettings struct {
	SrvAdr         string
	ReportInterval time.Duration
	PollInterval   time.Duration
}

func AgentConfig() AgentSettings {
	srvAdr := flag.String("a", "localhost:8080", "HTTP server address")
	reportInterval := flag.Int64("r", 10, "metrics report interval in seconds")
	pollInterval := flag.Int64("p", 2, "metrics poll interval in seconds")
	flag.Parse()

	return AgentSettings{
		SrvAdr:         *srvAdr,
		ReportInterval: time.Duration(*reportInterval) * time.Second,
		PollInterval:   time.Duration(*pollInterval) * time.Second,
	}
}
