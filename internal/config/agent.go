package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

type AgentSettings struct {
	Key            string `env:"KEY"`
	SrvAdr         string `env:"ADDRESS"`
	ReportInterval int64  `env:"REPORT_INTERVAL"`
	PollInterval   int64  `env:"POLL_INTERVAL"`
}

func AgentConfig() (AgentSettings, error) {
	var settings AgentSettings
	flag.StringVar(&settings.Key, "k", "", "key for HMAC-SHA256 request signing")
	flag.StringVar(&settings.SrvAdr, "a", "localhost:8080", "HTTP server address")
	flag.Int64Var(&settings.ReportInterval, "r", 10, "metrics report interval in seconds")
	flag.Int64Var(&settings.PollInterval, "p", 2, "metrics poll interval in seconds")
	flag.Parse()

	if err := env.Parse(&settings); err != nil {
		return AgentSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return settings, nil
}
