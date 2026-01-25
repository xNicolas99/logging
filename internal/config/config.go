package config

import (
	"encoding/json"
	"os"

	"github.com/jules/http-monitor/internal/model"
)

// InfluxConfig holds configuration for InfluxDB 2.0.
type InfluxConfig struct {
	URL    string `json:"url"`
	Token  string `json:"token"`
	Org    string `json:"org"`
	Bucket string `json:"bucket"`
}

// Config represents the application configuration.
type Config struct {
	Interval int            `json:"interval"` // Measurement interval in seconds
	Targets  []model.Target `json:"targets"`
	Influx   *InfluxConfig  `json:"influx,omitempty"`
}

// LoadConfig reads configuration from the specified file path.
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig writes configuration to the specified file path.
func SaveConfig(path string, cfg *Config) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(cfg)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Interval: 3600, // 60 minutes default
		Targets: []model.Target{
			{Name: "GitHub", URL: "https://github.com/", Threshold: 500000},
			{Name: "GHCR", URL: "https://ghcr.io/", Threshold: 500000},
			{Name: "Hetzner Speedtest", URL: "https://speed.hetzner.de/100MB.bin", Threshold: 5000000},
			{Name: "Cloudflare", URL: "https://1.1.1.1", Threshold: 500000},
		},
		Influx: &InfluxConfig{
			Org:    "myorg",
			Bucket: "monitor",
		},
	}
}
