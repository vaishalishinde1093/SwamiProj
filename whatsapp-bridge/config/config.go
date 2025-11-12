package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Database DatabaseConfig           `yaml:"database"`
	WhatsApp WhatsAppConfig           `yaml:"whatsapp"`
	Server   ServerConfig             `yaml:"server"`
	Groups   map[string][]GroupConfig `yaml:"groups"`
}

// GroupConfig represents a single seva group configuration
type GroupConfig struct {
	Number      int    `yaml:"number"`
	JID         string `yaml:"jid"`
	Name        string `yaml:"name"`
	CSVPath     string `yaml:"csv_path"`
	MaxAdhyas   int    `yaml:"max_adhyas"`
	MaxPollSize int    `yaml:"max_poll_size"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// WhatsAppConfig holds WhatsApp client configuration
type WhatsAppConfig struct {
	StorePath string `yaml:"store_path"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults if not specified
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8981
	}

	if cfg.Database.Path == "" {
		cfg.Database.Path = "store/messages.db"
	}

	if cfg.WhatsApp.StorePath == "" {
		cfg.WhatsApp.StorePath = "store/whatsapp.db"
	}

	return &cfg, nil
}
