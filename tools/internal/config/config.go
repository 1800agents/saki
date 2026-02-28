package config

import "os"

const (
	defaultAddr = "127.0.0.1:8080"
	defaultMode = "local"
)

// Config captures runtime settings for the local tool process.
type Config struct {
	Addr string
	Mode string
}

func Load() Config {
	cfg := Config{
		Addr: defaultAddr,
		Mode: defaultMode,
	}

	if v := os.Getenv("SAKI_TOOLS_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("SAKI_TOOLS_MODE"); v != "" {
		cfg.Mode = v
	}

	return cfg
}
