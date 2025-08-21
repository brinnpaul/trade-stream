package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Binance BinanceConfig `yaml:"binance"`
}

type BinanceConfig struct {
	ApiUrl           string   `yaml:"api_url"`
	WebsocketUrl     string   `yaml:"websocket_url"`
	Symbols          []string `yaml:"symbols"`
	SymbolsPerStream int      `yaml:"symbols_per_stream"`
}

func LoadConfig(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
}

func (c *Config) ShouldLoadAllSymbols() bool {
	return c.Binance.Symbols == nil || len(c.Binance.Symbols) == 0
}
