package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jules/http-monitor/internal/collector"
	"github.com/jules/http-monitor/internal/config"
	"github.com/jules/http-monitor/internal/logger"
	"github.com/jules/http-monitor/internal/monitor"
	"github.com/jules/http-monitor/internal/server"
	"github.com/jules/http-monitor/internal/storage"
)

func main() {
	// Default to /app/data/config.json for persistence
	configPath := flag.String("config", "/app/data/config.json", "Path to configuration file")
	flag.Parse()

	// Ensure data directory exists
	dataDir := filepath.Dir(*configPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Warning: Failed to create data directory %s: %v", dataDir, err)
	}

	// Load Configuration
	var cfg *config.Config
	var err error

	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Printf("Config file not found at %s. Creating default.", *configPath)
		cfg = config.DefaultConfig()

		if err := config.SaveConfig(*configPath, cfg); err != nil {
			log.Printf("Warning: Failed to save default config: %v", err)
		}
	} else {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	}

	// Override InfluxDB config from Env Vars (Docker Support)
	envURL := os.Getenv("INFLUX_URL")
	envToken := os.Getenv("INFLUX_TOKEN")
	envOrg := os.Getenv("INFLUX_ORG")
	envBucket := os.Getenv("INFLUX_BUCKET")

	if envURL != "" || envToken != "" {
		if cfg.Influx == nil {
			cfg.Influx = &config.InfluxConfig{}
		}
		if envURL != "" {
			cfg.Influx.URL = envURL
		}
		if envToken != "" {
			cfg.Influx.Token = envToken
		}
		if envOrg != "" {
			cfg.Influx.Org = envOrg
		}
		if envBucket != "" {
			cfg.Influx.Bucket = envBucket
		}
	}

	// Initialize Storage
	var store storage.Storage
	if cfg.Influx != nil && cfg.Influx.URL != "" && cfg.Influx.Token != "" {
		log.Printf("Using InfluxDB Storage at %s", cfg.Influx.URL)
		for {
			s, err := storage.NewInfluxStorage(cfg.Influx.URL, cfg.Influx.Token, cfg.Influx.Org, cfg.Influx.Bucket)
			if err == nil {
				store = s
				log.Println("Successfully connected to InfluxDB")
				break
			}
			log.Printf("Failed to connect to InfluxDB: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		}
	} else {
		log.Println("Using JSONL File Storage")
		// Use /app/data for JSONL storage too
		jsonlPath := filepath.Join(dataDir, "measurements.jsonl")
		s, err := storage.NewJSONLStorage(jsonlPath)
		if err != nil {
			log.Fatalf("Failed to initialize JSONL storage: %v", err)
		}
		store = s
	}
	defer store.Close()

	// Initialize Logger
	// Use /app/data for logs too for persistence
	logPath := filepath.Join(dataDir, "measurements.log")
	fileLogger, err := logger.NewFileLogger(logPath)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer fileLogger.Close()

	// Initialize Collector
	col := collector.NewCollector(store, fileLogger)

	// Initialize Monitor Service
	mon := monitor.NewMonitor(*configPath, cfg, col)
	mon.Start()
	defer mon.Stop()

	// Start Web Server
	srv := server.NewServer(cfg, store, mon, logPath)
	go func() {
		if err := srv.Start(8080); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Graceful Shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("\nShutting down...")
	// Allow some time for cleanup if needed
	time.Sleep(1 * time.Second)
}
