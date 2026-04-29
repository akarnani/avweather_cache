package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Metrics MetricsConfig `yaml:"metrics"`
	Cache   CacheConfig   `yaml:"cache"`
}

// ServerConfig represents server-specific configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// MetricsConfig represents Prometheus metrics listener configuration
type MetricsConfig struct {
	Port int `yaml:"port"`
}

// CacheConfig represents cache-specific configuration
type CacheConfig struct {
	UpdateInterval time.Duration `yaml:"update_interval"`
	SourceURL      string        `yaml:"source_url"`
}

// Load loads configuration from file and environment variables
// Environment variables take precedence over file configuration
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Metrics: MetricsConfig{
			Port: 9090,
		},
		Cache: CacheConfig{
			UpdateInterval: 5 * time.Minute,
			SourceURL:      "https://aviationweather.gov/data/cache/metars.cache.xml.gz",
		},
	}

	// Load from file if it exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables
	if port := os.Getenv("SERVER_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
		}
		cfg.Server.Port = p
	}

	if port := os.Getenv("METRICS_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_PORT: %w", err)
		}
		cfg.Metrics.Port = p
	}

	if interval := os.Getenv("CACHE_UPDATE_INTERVAL"); interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_UPDATE_INTERVAL: %w", err)
		}
		cfg.Cache.UpdateInterval = d
	}

	if url := os.Getenv("CACHE_SOURCE_URL"); url != "" {
		cfg.Cache.SourceURL = url
	}

	return cfg, nil
}
