package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
)

type AgentSettings struct {
	Key            string `env:"KEY"`
	ServerAddress  string `env:"ADDRESS"`
	ReportInterval int64  `env:"REPORT_INTERVAL"`
	PollInterval   int64  `env:"POLL_INTERVAL"`
	RateLimit      int64  `env:"RATE_LIMIT"`
}

func AgentConfig() (AgentSettings, error) {
	return parseAgentConfig(flag.CommandLine, os.Args[1:])
}

func parseAgentConfig(flags *flag.FlagSet, args []string) (AgentSettings, error) {
	var settings AgentSettings
	flags.StringVar(&settings.Key, "k", "", "key for HMAC-SHA256 request signing")
	flags.StringVar(&settings.ServerAddress, "a", "localhost:8080", "HTTP server address")
	flags.Int64Var(&settings.ReportInterval, "r", 10, "metrics report interval in seconds")
	flags.Int64Var(&settings.PollInterval, "p", 2, "metrics poll interval in seconds")
	flags.Int64Var(&settings.RateLimit, "l", 1, "maximum number of concurrent requests")
	if err := flags.Parse(args); err != nil {
		return AgentSettings{}, fmt.Errorf("parse flags: %w", err)
	}

	if err := env.Parse(&settings); err != nil {
		return AgentSettings{}, fmt.Errorf("parse env config: %w", err)
	}
	if settings.RateLimit < 1 {
		return AgentSettings{}, fmt.Errorf("rate limit must be positive: %d", settings.RateLimit)
	}

	return settings, nil
}
