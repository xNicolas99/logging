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
	// 1. Run HTTP Check FIRST (Condition MTR)
	// Use 20s timeout for download to allow for speed ramp up
	ctxDl, cancelDl := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelDl()

	req, err := http.NewRequestWithContext(ctxDl, "GET", t.URL, nil)
	if err != nil {
		c.logError(t, err, 0, "HTTP Request Failed")
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
		// If HTTP failed, we probably want to run MTR to see why
		// We can do it here or let the common logic handle it if we flag it?
		// Let's run MTR here to diagnose
		ctxMtr, cancelMtr := context.WithTimeout(context.Background(), 5*time.Second)
		loss, traceOutput := c.runMTR(ctxMtr, t.URL)
		cancelMtr()
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

	// Alerting Logic & Conditional MTR
	status := "OK"
	shouldRunMTR := false

	if isSpeedTest {
		// Speed Test Logic
		if speed < t.Threshold {
			status = "ALERT"
			shouldRunMTR = true // Analyze why speed is slow
		}
		// For SpeedTests, usually we might want MTR anyway?
		// User said: "Führe bei jeder Messung (egal ob Speed oder Latenz) parallel (oder direkt davor) einen Traceroute durch"
		// BUT later said: "Web Checks... only run MTR if check fails".
		// For SpeedTests, it implies we still want it? Or maybe conditionally too?
		// Let's assume for SpeedTests (heavy), we can afford the MTR or want it.
		// But to keep it "Conditional" based on performance feedback:
		// If Speed is OK, maybe skip?
		// However, "Packet Loss" graph relies on MTR data. If we skip MTR, we have 0 packet loss data.
		// If we skip MTR, the graph will show 0 loss.
		// User said: "Extrahiere den Packet Loss (%) ... und speichere ihn".
		// If we don't run MTR, we don't have this.
		// BUT user complains about "Web Checks too slow".
		// So for Web Checks (IsSpeedTest=false), we skip if healthy.
		// For Speed Tests, we probably should run it (or maybe fast version).
		// Let's run it for SpeedTests to populate the graph, as SpeedTests are slow anyway (seconds).
		shouldRunMTR = true
	} else {
		// Web Check Logic
		if statusCode != 200 {
			status = "ALERT"
			shouldRunMTR = true
		}
		if ttfb > 500 { // User suggested >500ms threshold for "high latency"
			status = "ALERT"
			shouldRunMTR = true
		}
	}

	var loss float64
	var traceOutput string

	if shouldRunMTR {
		// Run Optimized MTR
		// Use 5s timeout
		ctxMtr, cancelMtr := context.WithTimeout(context.Background(), 5*time.Second)
		loss, traceOutput = c.runMTR(ctxMtr, t.URL)
		cancelMtr()
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
		IsSpeedTest: false,
		Latency:     0,
	}
	c.storage.SaveMeasurement(m)
	c.logger.Log("ALERT", t.Name, 0, 0, 0)
}

// runMTR executes mtr --json --report-cycles 1 --no-dns <host>
func (c *Collector) runMTR(ctx context.Context, targetURL string) (float64, string) {
	// Extract hostname from URL
	u, err := url.Parse(targetURL)
	host := targetURL
	if err == nil && u.Host != "" {
		host = u.Host
	}

	// Optimized MTR:
	// --json: JSON Output
	// --report-cycles 1: Only 1 cycle (very fast)
	// --no-dns: Skip DNS resolution (faster)
	// Note: Alpine mtr might have different flags, but usually these are standard.
	// user suggested: mtr --report --report-cycles 1 --no-dns
	// We combine with --json for parsing.
	cmd := exec.CommandContext(ctx, "mtr", "--json", "--report-cycles", "1", "--no-dns", host)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Sprintf("MTR failed: %v", err)
	}

	// Parse JSON
	type MtrHop struct {
		Count int     `json:"count"`
		Loss  float64 `json:"Loss%"`
		Host  string  `json:"host"`
	}
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
