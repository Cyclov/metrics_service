package config

import "flag"

type ServerSettings struct {
	SrvAdr string
}

func ServerConfig() ServerSettings {
	srvAdr := flag.String("a", ":8080", "HTTP server address")
	flag.Parse()

	return ServerSettings{SrvAdr: *srvAdr}
}
