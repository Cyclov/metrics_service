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
	RateLimit      int    `env:"RATE_LIMIT"`
}

func AgentConfig() (AgentSettings, error) {
	var settings AgentSettings
	flag.StringVar(&settings.Key, "k", "", "key for HMAC-SHA256 request signing")
	flag.StringVar(&settings.SrvAdr, "a", "localhost:8080", "HTTP server address")
	flag.Int64Var(&settings.ReportInterval, "r", 10, "metrics report interval in seconds")
	flag.Int64Var(&settings.PollInterval, "p", 2, "metrics poll interval in seconds")
	flag.IntVar(&settings.RateLimit, "l", 1, "maximum number of concurrent requests")
	flag.Parse()

	if err := env.Parse(&settings); err != nil {
		return AgentSettings{}, fmt.Errorf("parse env config: %w", err)
	}
	if settings.RateLimit < 1 {
		return AgentSettings{}, fmt.Errorf("rate limit must be positive: %d", settings.RateLimit)
	}

	return settings, nil
}
