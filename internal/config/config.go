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
			{Name: "Hetzner 100MB", URL: "https://fsn1-speed.hetzner.com/100MB.bin", Threshold: 5000000, Interval: 60},
			{Name: "Hetzner 1GB", URL: "https://fsn1-speed.hetzner.com/1GB.bin", Threshold: 5000000, Interval: 60},
			{Name: "Cloudflare Speed", URL: "https://speed.cloudflare.com/__down?bytes=25000000", Threshold: 500000, Interval: 60},
			{Name: "Google", URL: "https://google.com", Threshold: 50000, Interval: 10},
		},
		Influx: &InfluxConfig{
			Org:    "myorg",
			Bucket: "monitor",
		},
	}
}

// EnsureMandatoryTargets adds required targets if they are missing.
func (c *Config) EnsureMandatoryTargets() bool {
	mandatory := []model.Target{
		{Name: "GitHub", URL: "https://github.com", Interval: 10, Threshold: 50000},
		{Name: "GHCR", URL: "https://ghcr.io", Interval: 10, Threshold: 50000},
		{Name: "Cloudflare", URL: "https://1.1.1.1", Interval: 1, Threshold: 50000},
		{Name: "Google DNS", URL: "https://8.8.8.8", Interval: 1, Threshold: 50000},
	}

	modified := false
	existingURLs := make(map[string]bool)
	for _, t := range c.Targets {
		existingURLs[t.URL] = true
	}

	for _, m := range mandatory {
		if !existingURLs[m.URL] {
			c.Targets = append(c.Targets, m)
			modified = true
			existingURLs[m.URL] = true
		}
	}
	return modified
}
