package config

import (
	"fmt"
	"github.com/caarlos0/env/v7"
)

var c *Config

func C() *Config { return c }

func Init() error {
	c = new(Config)
	err := env.Parse(c)
	if err != nil {
		return fmt.Errorf("parse env: %w", err)
	}

	return nil
}

type Config struct {
	SaSets     ServerAConfig `envPrefix:"sa_"`
	SbSets     ServerBConfig `envPrefix:"sb_"`
	ClientSets ClientsConfig `envPrefix:"client_"`
}

type ClientsConfig struct {
	ServerAURL string `env:"sb_url"`
	ServerBURL string `env:"sa_url"`
}

type ServerBConfig struct {
	Host    string `env:"host"`
	Port    int    `env:"port"`
	LogPath string `env:"log_path"`
}

type ServerAConfig struct {
	Host    string `env:"host"`
	Port    int    `env:"port"`
	LogPath string `env:"log_path"`
}
