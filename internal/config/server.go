package config

import (
	"flag"

	"github.com/caarlos0/env/v11"
)

type ServerSettings struct {
	SrvAdr string `env:"ADDRESS"`
}

func ServerConfig() ServerSettings {
	srvAdr := flag.String("a", ":8080", "HTTP server address")
	flag.Parse()

	settings := ServerSettings{SrvAdr: *srvAdr}
	_ = env.Parse(&settings)

	return settings
}
