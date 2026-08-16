package config

import (
	"flag"
	"time"

	"github.com/caarlos0/env/v11"
)

type ServerSettings struct {
	SrvAdr          string
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
}

func ServerConfig() ServerSettings {
	srvAdr := flag.String("a", ":8080", "HTTP server address")
	storeInterval := flag.Int64("i", 300, "metrics store interval in seconds")
	fileStoragePath := flag.String("f", "metrics.json", "metrics storage file path")
	restore := flag.Bool("r", true, "restore metrics from storage file")
	flag.Parse()

	environment := struct {
		SrvAdr          string `env:"ADDRESS"`
		StoreInterval   int64  `env:"STORE_INTERVAL"`
		FileStoragePath string `env:"FILE_STORAGE_PATH"`
		Restore         bool   `env:"RESTORE"`
	}{
		SrvAdr:          *srvAdr,
		StoreInterval:   *storeInterval,
		FileStoragePath: *fileStoragePath,
		Restore:         *restore,
	}
	_ = env.Parse(&environment)

	return ServerSettings{
		SrvAdr:          environment.SrvAdr,
		StoreInterval:   time.Duration(environment.StoreInterval) * time.Second,
		FileStoragePath: environment.FileStoragePath,
		Restore:         environment.Restore,
	}
}
