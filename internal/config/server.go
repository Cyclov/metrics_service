package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type ServerSettings struct {
	SrvAdr          string        `env:"ADDRESS"`
	StoreInterval   time.Duration `env:"STORE_INTERVAL"`
	FileStoragePath string        `env:"FILE_STORAGE_PATH"`
	Restore         bool          `env:"RESTORE"`
}

func ServerConfig() (ServerSettings, error) {
	var cfg ServerSettings
	flag.StringVar(&cfg.SrvAdr, "a", ":8080", "HTTP server address")
	flag.DurationVar(&cfg.StoreInterval, "i", 300*time.Second, "metrics store interval")
	flag.StringVar(&cfg.FileStoragePath, "f", "metrics.json", "metrics storage file path")
	flag.BoolVar(&cfg.Restore, "r", true, "restore metrics from storage file")
	flag.Parse()

	if err := env.Parse(&cfg); err != nil {
		return ServerSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return cfg, nil
}
