package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
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
	// 1. Run MTR / Ping Check (Sequential, BEFORE Speedtest)
	// Use 15s timeout for MTR
	ctxMtr, cancelMtr := context.WithTimeout(context.Background(), 15*time.Second)
	loss, traceOutput := c.runMTR(ctxMtr, t.URL)
	cancelMtr()

	// 2. Download Speed Test / Web Check
	// Use 20s timeout for download to allow for speed ramp up
	ctxDl, cancelDl := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelDl()

	req, err := http.NewRequestWithContext(ctxDl, "GET", t.URL, nil)
	if err != nil {
		c.logError(t, err, loss, traceOutput)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			ForceAttemptHTTP2: true,
		},
	}
	req.Header.Set("User-Agent", "HTTP-Monitor/1.0")

	start := time.Now()
	resp, err := client.Do(req)

	// Measure TTFB (Time To First Byte / Headers)
	ttfb := time.Since(start).Seconds() * 1000 // in ms

	var written int64
	var totalDuration float64
	var dlErr error
	var statusCode int

	if err != nil {
		dlErr = err
	} else {
		statusCode = resp.StatusCode
		defer resp.Body.Close()

		// Limit download to 500MB
		const maxBytes = 500 * 1024 * 1024
		reader := io.LimitReader(resp.Body, maxBytes)

		// Use CopyBuffer
		buf := make([]byte, 32*1024)
		written, dlErr = io.CopyBuffer(io.Discard, reader, buf)
		totalDuration = time.Since(start).Seconds()

		// Ignore context deadline exceeded or EOF if we got some data
		if dlErr == context.DeadlineExceeded || dlErr == io.EOF || dlErr == io.ErrUnexpectedEOF {
			dlErr = nil
		}
	}

	if dlErr != nil {
		c.logError(t, dlErr, loss, traceOutput)
		return
	}

	// Calculate Speed
	speed := 0.0
	if totalDuration > 0 {
		speed = float64(written) / totalDuration
	}

	// Determine IsSpeedTest
	// Rule: TotalSize > 5 MB ODER Duration > 2 Seconds
	isSpeedTest := false
	if written > 5*1024*1024 || totalDuration > 2.0 {
		isSpeedTest = true
	}

	// Alerting Logic
	status := "OK"
	if isSpeedTest {
		// Speed Test Logic: Speed Drop + Packet Loss (Packet Loss handled by looking at the loss value?
		// Original logic only checked speed.
		// "Speed Tests (IsSpeedTest=true): Behalte die bestehende Logik (Speed Drop + Packet Loss Alerts) bei."
		// Existing logic was: if speed < t.Threshold { status = "ALERT" }
		if speed < t.Threshold {
			status = "ALERT"
		}
	} else {
		// Web Check Logic
		// "Sende Alerts nur, wenn HTTP Status != 200 ODER die Latenz > 1000ms"
		if statusCode != 200 {
			status = "ALERT"
		}
		if ttfb > 1000 {
			status = "ALERT"
		}
	}

	m := model.Measurement{
		Timestamp:   start,
		Target:      t.Name,
		Duration:    totalDuration,
		Size:        written,
		Speed:       speed,
		Status:      status,
		PacketLoss:  loss,
		TraceOutput: traceOutput,
		IsSpeedTest: isSpeedTest,
		Latency:     ttfb,
	}

	// Save to storage
	if err := c.storage.SaveMeasurement(m); err != nil {
		fmt.Printf("Error saving measurement for %s: %v\n", t.Name, err)
	}

	// Log to file (Legacy logging format, maybe we want to add latency/isSpeedTest here too?
	// The logger interface assumes fixed args. I will keep it as is for compatibility or update it if I updated logger.)
	// I didn't update Logger signature in plan. I will just log speed/duration.
	c.logger.Log(status, t.Name, totalDuration, written, speed)
}

func (c *Collector) logError(t model.Target, err error, loss float64, traceOutput string) {
	m := model.Measurement{
		Timestamp:   time.Now(),
		Target:      t.Name,
		Duration:    0,
		Size:        0,
		Speed:       0,
		Status:      "ALERT",
		PacketLoss:  loss,
		TraceOutput: fmt.Sprintf("Error: %v\n%s", err, traceOutput),
		IsSpeedTest: false, // Error usually means web check failed or connect failed
		Latency:     0,
	}
	c.storage.SaveMeasurement(m)
	c.logger.Log("ALERT", t.Name, 0, 0, 0)
}

// runMTR executes mtr -j -c 10 <host> and returns loss percentage and full output
func (c *Collector) runMTR(ctx context.Context, targetURL string) (float64, string) {
	// Extract hostname from URL
	u, err := url.Parse(targetURL)
	host := targetURL
	if err == nil && u.Host != "" {
		host = u.Host
	}

	// mtr needs root, usually available in docker
	// -j: JSON output
	// -c 10: 10 cycles
	// Note: mtr -j outputs the report at the end.
	cmd := exec.CommandContext(ctx, "mtr", "--json", "-c", "10", host)
	out, err := cmd.Output()
	if err != nil {
		// If mtr fails (e.g. not installed or permission), return 0 loss and error as trace
		return 0, fmt.Sprintf("MTR failed: %v", err)
	}

	// Parse JSON
	type MtrHop struct {
		Count int     `json:"count"`
		Loss  float64 `json:"Loss%"`
		Host  string  `json:"host"`
	}
	// mtr 0.92+ uses "report": { "hubs": ... }
	type Root struct {
		Report struct {
			Hubs []MtrHop `json:"hubs"`
		} `json:"report"`
	}

	var res Root
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, string(out) // Return raw output if parse fails
	}

	hubs := res.Report.Hubs
	if len(hubs) == 0 {
		return 0, string(out)
	}

	// Last hop is usually the destination
	lastHop := hubs[len(hubs)-1]

	return lastHop.Loss, string(out)
}
