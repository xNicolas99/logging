package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jules/http-monitor/internal/collector"
	"github.com/jules/http-monitor/internal/config"
	"github.com/jules/http-monitor/internal/logger"
	"github.com/jules/http-monitor/internal/server"
	"github.com/jules/http-monitor/internal/storage"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load Configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override InfluxDB config from Env Vars (Docker Support)
	if url := os.Getenv("INFLUX_URL"); url != "" {
		if cfg.Influx == nil {
			cfg.Influx = &config.InfluxConfig{}
		}
		cfg.Influx.URL = url
	}
	if token := os.Getenv("INFLUX_TOKEN"); token != "" {
		if cfg.Influx == nil {
			log.Fatal("INFLUX_TOKEN env var set but InfluxDB not configured in config file structure properly (or use full config). Assuming basic struct.")
			cfg.Influx = &config.InfluxConfig{}
		}
		cfg.Influx.Token = token
	}
	if org := os.Getenv("INFLUX_ORG"); org != "" {
		if cfg.Influx != nil {
			cfg.Influx.Org = org
		}
	}
	if bucket := os.Getenv("INFLUX_BUCKET"); bucket != "" {
		if cfg.Influx != nil {
			cfg.Influx.Bucket = bucket
		}
	}

	// Initialize Storage
	var store storage.Storage
	if cfg.Influx != nil && cfg.Influx.URL != "" && cfg.Influx.Token != "" {
		log.Printf("Using InfluxDB Storage at %s", cfg.Influx.URL)
		s, err := storage.NewInfluxStorage(cfg.Influx.URL, cfg.Influx.Token, cfg.Influx.Org, cfg.Influx.Bucket)
		if err != nil {
			log.Fatalf("Failed to initialize InfluxDB: %v", err)
		}
		store = s
	} else {
		log.Println("Using JSONL File Storage")
		// Ensure data dir exists
		os.MkdirAll("data", 0755)
		s, err := storage.NewJSONLStorage("data/measurements.jsonl")
		if err != nil {
			log.Fatalf("Failed to initialize JSONL storage: %v", err)
		}
		store = s
	}
	defer store.Close()

	// Initialize Logger
	logPath := "logs/measurements.log"
	fileLogger, err := logger.NewFileLogger(logPath)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer fileLogger.Close()

	// Initialize Collector
	col := collector.NewCollector(store, fileLogger)

	// Start Background Collection
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	quit := make(chan struct{})
	go func() {
		// Run once immediately
		for _, t := range cfg.Targets {
			go col.MeasureTarget(t)
		}
		for {
			select {
			case <-ticker.C:
				for _, t := range cfg.Targets {
					// Launch each measurement in a goroutine to avoid blocking
					go col.MeasureTarget(t)
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	// Start Web Server
	srv := server.NewServer(cfg, store, logPath)
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
	close(quit)
	// Allow some time for cleanup if needed
	time.Sleep(1 * time.Second)
}
