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
	var settings AgentSettings
	flag.StringVar(&settings.SrvAdr, "a", "localhost:8080", "HTTP server address")
	flag.DurationVar(&settings.ReportInterval, "r", 10*time.Second, "metrics report interval")
	flag.DurationVar(&settings.PollInterval, "p", 2*time.Second, "metrics poll interval")
	flag.Parse()

	if err := env.Parse(&settings); err != nil {
		return AgentSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return settings, nil
}
