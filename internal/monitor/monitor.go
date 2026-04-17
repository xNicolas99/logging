package monitor

import (
	"log"
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
	jobQueue   chan model.Target
	running    bool
	mu         sync.Mutex
	wg         sync.WaitGroup
	lastRun    map[string]time.Time
}

func NewMonitor(configPath string, cfg *config.Config, col *collector.Collector) *Monitor {
	return &Monitor{
		cfg:        cfg,
		configPath: configPath,
		collector:  col,
		lastRun:    make(map[string]time.Time),
		jobQueue:   make(chan model.Target, 100), // Buffer to allow some buildup
	}
}

func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.quit = make(chan struct{})
	m.running = true
	m.wg.Add(2) // runLoop + worker

	// Start worker
	go m.worker()

	// Check every 10 seconds
	go m.runLoop(10 * time.Second)
}

func (m *Monitor) worker() {
	defer m.wg.Done()
	for {
		select {
		case t := <-m.jobQueue:
			m.collector.MeasureTarget(t)
		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) runLoop(checkInterval time.Duration) {
	defer m.wg.Done()
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Initial check
	m.checkAndRun()

	for {
		select {
		case <-ticker.C:
			m.checkAndRun()
		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) checkAndRun() {
	m.mu.Lock()
	// Early exit if stopping
	if !m.running {
		m.mu.Unlock()
		return
	}

	targets := make([]model.Target, len(m.cfg.Targets))
	copy(targets, m.cfg.Targets)
	globalIntervalSec := m.cfg.Interval
	m.mu.Unlock()

	for _, t := range targets {
		// Calculate target interval duration
		var interval time.Duration
		if t.Interval > 0 {
			interval = time.Duration(t.Interval) * time.Minute
		} else {
			interval = time.Duration(globalIntervalSec) * time.Second
		}

		// Enforce minimum 1 minute to avoid crazy loops if config is bad
		if interval < 1*time.Minute {
			interval = 1 * time.Minute
		}

		m.mu.Lock()
		last := m.lastRun[t.Name]
		m.mu.Unlock()

		if time.Since(last) >= interval {
			// Trigger measurement
			// Update lastRun immediately to prevent double triggering
			m.mu.Lock()
			m.lastRun[t.Name] = time.Now()
			m.mu.Unlock()

			// Enqueue
			select {
			case m.jobQueue <- t:
			default:
				log.Printf("Warning: Job queue full, skipping check for %s", t.Name)
			}
		}
	}
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.quit)
	// Do NOT close jobQueue. It prevents panic in concurrent writers.
	// The worker will exit when it sees quit closed.
}

// RunNow triggers an immediate measurement for all targets.
func (m *Monitor) RunNow() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	targets := make([]model.Target, len(m.cfg.Targets))
	copy(targets, m.cfg.Targets)

	// Update lastRun for all
	now := time.Now()
	for _, t := range targets {
		m.lastRun[t.Name] = now
	}
	m.mu.Unlock()

	for _, t := range targets {
		select {
		case m.jobQueue <- t:
		default:
			log.Printf("Warning: Job queue full, skipping manual run for %s", t.Name)
		}
	}
}

// AddTarget adds a new target to the configuration and saves it.
func (m *Monitor) AddTarget(name, url string, interval int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := model.Target{
		Name:      name,
		URL:       url,
		Threshold: 500000, // Default threshold
		Interval:  interval,
	}
	m.cfg.Targets = append(m.cfg.Targets, t)

	return config.SaveConfig(m.configPath, m.cfg)
}

// DeleteTarget removes a target by name.
func (m *Monitor) DeleteTarget(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newTargets := []model.Target{}
	found := false
	for _, t := range m.cfg.Targets {
		if t.Name == name {
			found = true
			continue
		}
		newTargets = append(newTargets, t)
	}

	if !found {
		return nil // Or error "not found"
	}

	m.cfg.Targets = newTargets
	delete(m.lastRun, name)

	return config.SaveConfig(m.configPath, m.cfg)
}

// SetInterval updates the GLOBAL default measurement interval (in minutes).
func (m *Monitor) SetInterval(minutes int) error {
	if minutes < 1 {
		minutes = 1
	}

	m.mu.Lock()
	m.cfg.Interval = minutes * 60
	err := config.SaveConfig(m.configPath, m.cfg)
	m.mu.Unlock()

	// No need to restart loop, it picks up new interval in next tick
	return err
}
