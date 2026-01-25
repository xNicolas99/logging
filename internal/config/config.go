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
	Interval int            `json:"interval"` // Global default measurement interval in seconds
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
			{Name: "Hetzner Speedtest", URL: "https://fsn1-speed.hetzner.com/100MB.bin", Threshold: 5000000, Interval: 60},
			{Name: "Hetzner 1GB", URL: "https://fsn1-speed.hetzner.com/1GB.bin", Threshold: 5000000, Interval: 60},
			{Name: "Cloudflare", URL: "https://speed.cloudflare.com/__down?bytes=25000000", Threshold: 500000, Interval: 60},
			{Name: "Google", URL: "https://google.com", Threshold: 50000, Interval: 10},
		},
		Influx: &InfluxConfig{
			Org:    "myorg",
			Bucket: "monitor",
		},
	}
}
