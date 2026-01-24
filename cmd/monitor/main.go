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
