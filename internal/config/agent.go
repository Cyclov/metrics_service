package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type AgentSettings struct {
	SrvAdr         string        `env:"ADDRESS"`
	ReportInterval time.Duration `env:"REPORT_INTERVAL"`
	PollInterval   time.Duration `env:"POLL_INTERVAL"`
}

func AgentConfig() (AgentSettings, error) {
	srvAdr := flag.String("a", "localhost:8080", "HTTP server address")
	reportInterval := flag.Int64("r", 10, "metrics report interval in seconds")
	pollInterval := flag.Int64("p", 2, "metrics poll interval in seconds")
	flag.Parse()

	settings := AgentSettings{
		SrvAdr:         *srvAdr,
		ReportInterval: time.Duration(*reportInterval) * time.Second,
		PollInterval:   time.Duration(*pollInterval) * time.Second,
	}

	if err := env.Parse(&settings); err != nil {
		return AgentSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return settings, nil
}
