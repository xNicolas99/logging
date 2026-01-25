package monitor

import (
	"sync"
	"time"

	"github.com/jules/http-monitor/internal/collector"
	"github.com/jules/http-monitor/internal/config"
	"github.com/jules/http-monitor/internal/model"
)

type Monitor struct {
	cfg        *config.Config
	configPath string
	collector  *collector.Collector
	quit       chan struct{}
	running    bool
	mu         sync.Mutex
	wg         sync.WaitGroup
}

func NewMonitor(configPath string, cfg *config.Config, col *collector.Collector) *Monitor {
	return &Monitor{
		cfg:        cfg,
		configPath: configPath,
		collector:  col,
	}
}

func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	intervalSeconds := m.cfg.Interval
	// Enforce minimum 3 minutes (180 seconds)
	if intervalSeconds < 180 {
		intervalSeconds = 180
	}

	m.quit = make(chan struct{})
	m.running = true
	m.wg.Add(1)

	go m.runLoop(time.Duration(intervalSeconds) * time.Second)
}

func (m *Monitor) runLoop(interval time.Duration) {
	defer m.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately
	m.RunNow()

	for {
		select {
		case <-ticker.C:
			m.RunNow()
		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.quit)
	m.running = false
}

// RunNow triggers an immediate measurement for all targets.
func (m *Monitor) RunNow() {
	m.mu.Lock()
	// Copy targets to avoid holding lock during iteration or async calls if targets change
	targets := make([]model.Target, len(m.cfg.Targets))
	copy(targets, m.cfg.Targets)
	m.mu.Unlock()

	for _, t := range targets {
		// Launch each measurement in a goroutine
		go m.collector.MeasureTarget(t)
	}
}

// AddTarget adds a new target to the configuration and saves it.
func (m *Monitor) AddTarget(name, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := model.Target{
		Name:      name,
		URL:       url,
		Threshold: 500000, // Default threshold
	}
	m.cfg.Targets = append(m.cfg.Targets, t)

	return config.SaveConfig(m.configPath, m.cfg)
}

// SetInterval updates the measurement interval (in minutes) and saves configuration.
// If the monitor is running, it restarts the loop with the new interval.
func (m *Monitor) SetInterval(minutes int) error {
	if minutes < 3 {
		minutes = 3
	}

	m.mu.Lock()
	m.cfg.Interval = minutes * 60
	err := config.SaveConfig(m.configPath, m.cfg)
	isRunning := m.running
	m.mu.Unlock()

	if err != nil {
		return err
	}

	if isRunning {
		m.Stop()
		m.wg.Wait() // Wait for the old loop to exit completely
		m.Start()
	}

	return nil
}
