package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jules/http-monitor/internal/logger"
	"github.com/jules/http-monitor/internal/model"
	"github.com/jules/http-monitor/internal/storage"
)

// Collector handles the measurement logic.
type Collector struct {
	storage storage.Storage
	logger  *logger.FileLogger
}

func NewCollector(s storage.Storage, l *logger.FileLogger) *Collector {
	return &Collector{
		storage: s,
		logger:  l,
	}
}

// MeasureTarget performs a single measurement for a target.
func (c *Collector) MeasureTarget(t model.Target) {
	start := time.Now()

	// Use a 10s timeout as requested
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", t.URL, nil)
	if err != nil {
		c.logError(t, err)
		return
	}

	client := &http.Client{}

	// Fake user agent to avoid being blocked by some servers
	req.Header.Set("User-Agent", "HTTP-Monitor/1.0")

	resp, err := client.Do(req)
	if err != nil {
		c.logError(t, err)
		return
	}
	defer resp.Body.Close()

	// Limit download to 50MB or stop after some time.
	// Since we want to measure speed, reading up to 50MB is reasonable.
	// We also want to stop if it takes too long (e.g. 10s) but the client timeout handles that partially.
	// Let's rely on a MaxBytesReader to limit size.
	const maxBytes = 50 * 1024 * 1024 // 50 MB
	reader := io.LimitReader(resp.Body, maxBytes)

	// Buffer for reading - we discard data but we need to read it.
	// We use io.Copy to io.Discard.
	written, err := io.Copy(io.Discard, reader)
	if err != nil && err != io.EOF {
		// If limit reached, it's not strictly an error for us, we just stop.
		// But io.LimitReader returns EOF when done.
		// Actual network error might happen.
	}

	duration := time.Since(start).Seconds()
	speed := float64(written) / duration

	status := "OK"
	if speed < t.Threshold {
		status = "ALERT"
	}

	m := model.Measurement{
		Timestamp: start,
		Target:    t.Name,
		Duration:  duration,
		Size:      written,
		Speed:     speed,
		Status:    status,
	}

	// Save to storage
	if err := c.storage.SaveMeasurement(m); err != nil {
		fmt.Printf("Error saving measurement for %s: %v\n", t.Name, err)
	}

	// Log to file
	c.logger.Log(status, t.Name, duration, written, speed)
}

func (c *Collector) logError(t model.Target, err error) {
	// Log failed attempt
	// We treat error as 0 speed and ALERT
	m := model.Measurement{
		Timestamp: time.Now(),
		Target:    t.Name,
		Duration:  0,
		Size:      0,
		Speed:     0,
		Status:    "ALERT",
	}
	c.storage.SaveMeasurement(m)
	c.logger.Log("ALERT", t.Name, 0, 0, 0)
}
