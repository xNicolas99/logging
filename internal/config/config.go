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
