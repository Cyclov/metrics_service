package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v11"
)

type ServerSettings struct {
	SrvAdr          string `env:"ADDRESS"`
	DBAdr           string `env:"DATABASE_DSN"`
	StoreInterval   int64  `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Key             string `env:"KEY"`
	Restore         bool   `env:"RESTORE"`
}

func ServerConfig() (ServerSettings, error) {
	var settings ServerSettings

	flag.StringVar(&settings.SrvAdr, "a", ":8080", "HTTP server address")
	flag.StringVar(&settings.DBAdr, "d", "", "database connection string")
	flag.Int64Var(&settings.StoreInterval, "i", 300, "metrics store interval in seconds")
	flag.StringVar(&settings.FileStoragePath, "f", "", "metrics storage file path")
	flag.StringVar(&settings.Key, "k", "", "key for HMAC-SHA256 request signing")
	flag.BoolVar(&settings.Restore, "r", true, "restore metrics from storage file")
	flag.Parse()

	if err := env.Parse(&settings); err != nil {
		return ServerSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return settings, nil
}
