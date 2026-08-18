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
	var settings ServerSettings

	srvAdr := flag.String("a", ":8080", "HTTP server address")
	storeInterval := flag.Int64("i", 300, "metrics store interval in seconds")
	fileStoragePath := flag.String("f", "metrics.json", "metrics storage file path")
	restore := flag.Bool("r", true, "restore metrics from storage file")
	flag.Parse()

	settings.SrvAdr = *srvAdr
	settings.StoreInterval = time.Duration(*storeInterval) * time.Second
	settings.FileStoragePath = *fileStoragePath
	settings.Restore = *restore

	if err := env.Parse(&settings); err != nil {
		return ServerSettings{}, fmt.Errorf("parse env config: %w", err)
	}

	return settings, nil
}
