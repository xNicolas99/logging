package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
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
	var wg sync.WaitGroup

	// Traceroute result
	var loss float64
	var traceOutput string

	// Use 15s timeout to allow reaching high speeds and mtr to finish
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Start traceroute in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		loss, traceOutput = c.runMTR(ctx, t.URL)
	}()

	// Download Speed Test
	req, err := http.NewRequestWithContext(ctx, "GET", t.URL, nil)
	if err != nil {
		c.logError(t, err)
		wg.Wait() // wait for trace to finish even if download fails
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			ForceAttemptHTTP2: true,
		},
	}
	req.Header.Set("User-Agent", "HTTP-Monitor/1.0")

	resp, err := client.Do(req)

	var written int64
	var duration float64
	var dlErr error

	if err != nil {
		dlErr = err
	} else {
		defer resp.Body.Close()

		// Limit download to 500MB (approx 5s on 1Gbps) to avoid endless download,
		// but allow enough time for speed to ramp up.
		// If connection is 100Mbps, 500MB takes 40s -> timeout hits at 15s.
		// If connection is 1Gbps, 500MB takes 4s -> finishes early.
		const maxBytes = 500 * 1024 * 1024
		reader := io.LimitReader(resp.Body, maxBytes)

		// Use CopyBuffer with 32KB buffer for better performance than default
		buf := make([]byte, 32*1024)
		written, dlErr = io.CopyBuffer(io.Discard, reader, buf)
		duration = time.Since(start).Seconds()

		// Ignore context deadline exceeded or EOF if we got some data
		if dlErr == context.DeadlineExceeded || dlErr == io.EOF || dlErr == io.ErrUnexpectedEOF {
			dlErr = nil
		}
	}

	// Wait for traceroute
	wg.Wait()

	if dlErr != nil {
		c.logError(t, dlErr)
		return
	}

	speed := 0.0
	if duration > 0 {
		speed = float64(written) / duration
	}

	status := "OK"
	if speed < t.Threshold {
		status = "ALERT"
	}

	m := model.Measurement{
		Timestamp:   start,
		Target:      t.Name,
		Duration:    duration,
		Size:        written,
		Speed:       speed,
		Status:      status,
		PacketLoss:  loss,
		TraceOutput: traceOutput,
	}

	// Save to storage
	if err := c.storage.SaveMeasurement(m); err != nil {
		fmt.Printf("Error saving measurement for %s: %v\n", t.Name, err)
	}

	// Log to file
	c.logger.Log(status, t.Name, duration, written, speed)
}

func (c *Collector) logError(t model.Target, err error) {
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

// runMTR executes mtr -j -c 10 -r <host> and returns loss percentage and full output
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
	// -w: report wide (implies -r report mode) - actually -w is wide report.
	// user suggested: mtr -r -c 10 -w <host> OR mtr -j
	// We use -j for parsing. -z for no DNS lookup might be faster but we probably want DNS.
	// -w not always compatible with -j in some versions?
	// Let's try: mtr -j -c 10 <host> (this usually streams JSON or produces report at end?)
	// mtr --json creates a report at the end.

	cmd := exec.CommandContext(ctx, "mtr", "--json", "-c", "10", host)
	out, err := cmd.Output()
	if err != nil {
		// If mtr fails (e.g. not installed or permission), return 0 loss and error as trace
		return 0, fmt.Sprintf("MTR failed: %v", err)
	}

	// Parse JSON
	// Structure: { "report": { "hub": [ ... ] } }
	type MtrHop struct {
		Count int     `json:"count"`
		Loss  float64 `json:"Loss%"`
		Host  string  `json:"host"`
	}
	type MtrReport struct {
		Mtr struct {
			Src   string   `json:"src"`
			Dst   string   `json:"dst"`
			Tests int      `json:"tests"`
		} `json:"mtr"` // Sometimes it is nested differently or "report"
		Hubs []MtrHop `json:"hubs"`
	}

	// MTR JSON format can vary. Let's look at standard mtr --json output.
	// Typically: { "report": { "hubs": [...] } }
	// But let's handle a generic map to be safe first or try strict struct.
	// For simplicity, let's decode to map[string]interface{} to inspect.

	// NOTE: mtr 0.92+ uses "report": { "hubs": ... }
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
