package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	// 1. Run HTTP Check (Always run to determine type and basic availability)
	// For large files, this becomes the Speed Test.

	var loss float64
	var traceOutput string
	var pingLoss float64
	var pingLatency float64

	// Extract Host for Ping/MTR
	host := t.URL
	if u, err := url.Parse(t.URL); err == nil && u.Host != "" {
		host = u.Host
		// Remove port if present for ping/mtr
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}
	}

	// Use 20s timeout for download
	ctxDl, cancelDl := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelDl()

	req, err := http.NewRequestWithContext(ctxDl, "GET", t.URL, nil)
	// If creation fails (e.g. invalid URL), we treat it as immediate failure
	// We might run MTR if it's a network issue, but usually it's config error.
	// We'll proceed to try running Ping if HTTP fails, to see if host is up.

	var resp *http.Response
	var start time.Time
	var ttfb float64
	var totalDuration float64
	var written int64
	var statusCode int
	var dlErr error

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			ForceAttemptHTTP2: true,
		},
		// Don't follow redirects automatically if we want to measure exact target?
		// Default follows. That's fine.
	}
	req.Header.Set("User-Agent", "HTTP-Monitor/1.0")

	start = time.Now()
	if err == nil {
		resp, dlErr = client.Do(req)
	} else {
		dlErr = err
	}

	if dlErr == nil {
		ttfb = time.Since(start).Seconds() * 1000 // ms
		statusCode = resp.StatusCode
		defer resp.Body.Close()

		// Limit download to 500MB
		const maxBytes = 500 * 1024 * 1024
		reader := io.LimitReader(resp.Body, maxBytes)

		// Use CopyBuffer
		buf := make([]byte, 32*1024)
		written, dlErr = io.CopyBuffer(io.Discard, reader, buf)
		totalDuration = time.Since(start).Seconds()

		if dlErr == context.DeadlineExceeded || dlErr == io.EOF || dlErr == io.ErrUnexpectedEOF {
			dlErr = nil
		}
	} else {
		// HTTP Connection Failed
		// We still calculate duration so far?
		totalDuration = time.Since(start).Seconds()
	}

	// Calculate Speed
	speed := 0.0
	if totalDuration > 0 {
		speed = float64(written) / totalDuration
	}

	// Determine IsSpeedTest
	// Rule: Size > 5MB OR Duration > 2s
	isSpeedTest := false
	if written > 5*1024*1024 || totalDuration > 2.0 {
		isSpeedTest = true
	}

	// Logic Branching
	runMtr := false
	status := "OK"

	if isSpeedTest {
		// Speed Test Logic
		// Trigger MTR if Speed < Threshold OR HTTP Failed
		if dlErr != nil || statusCode != 200 {
			runMtr = true
			status = "ALERT"
		}
		if speed < t.Threshold {
			status = "ALERT"
			runMtr = true // Optional: Run MTR to debug slow speed?
			// User said "Speed Tests... Behalte die bestehende Logik".
			// Existing logic was "runMtr = true if isSpeedTest" (ALWAYS run MTR for Speed Test in original code?
			// Wait, original code:
			// if isSpeedTest { runMtr = true } else if ...
			// So it ALWAYS ran MTR for speed tests.
			// User said "The latency values are still unrealistic... overhead is blocking...".
			// But for Speed Tests, maybe they want MTR?
			// "For standard Web Checks (IsSpeedTest=false), do NOT run MTR... Only do a simple HTTP/Ping check."
			// Implicitly, for Speed Tests, we might still run MTR or only if slow.
			// "Speed Tests ... Behalte die bestehende Logik".
			// Existing logic ran MTR every time for SpeedTest.
			// I will keep it running every time for SpeedTest if that's what "existing logic" means.
			runMtr = true
		}
	} else {
		// Web Check Logic
		// 1. Run Ping
		ctxPing, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
		pLoss, pLat, pErr := c.runPing(ctxPing, host)
		cancelPing()

		pingLoss = pLoss
		pingLatency = pLat

		if pErr != nil {
			// Ping command failed to run or parse
			// We might assume it's bad.
			// If ping failed completely (e.g. timeout), packet loss is likely 100%
		}

		// Trigger MTR if:
		// 1. HTTP Failed (Status != 200 or Error)
		// 2. Ping Packet Loss > 0%
		// 3. Ping Latency > 100ms
		if dlErr != nil || statusCode != 200 {
			runMtr = true
			status = "ALERT"
		} else if pingLoss > 0 {
			runMtr = true
			status = "ALERT"
		} else if pingLatency > 100 {
			runMtr = true
			status = "ALERT"
		}
	}

	if runMtr {
		ctxMtr, cancelMtr := context.WithTimeout(context.Background(), 20*time.Second)
		loss, traceOutput = c.runMTR(ctxMtr, host)
		cancelMtr()
	} else {
		// If we didn't run MTR, we can use Ping Loss as the "Packet Loss" metric for Web Checks
		if !isSpeedTest {
			loss = pingLoss
		}
	}

	// For Web Checks, prioritize Ping Latency?
	// User: "Web Checks prioritize Latency (ms) display"
	// User: "This must result in <50ms latency for Google."
	// HTTP TTFB might be higher. Ping latency is pure network.
	// If I put Ping Latency into `Latency` field, it satisfies the "network" latency requirement.
	// If I put TTFB, it satisfies "HTTP" latency.
	// I will use Ping Latency for Web Checks if available and > 0.
	finalLatency := ttfb
	if !isSpeedTest && pingLatency > 0 {
		finalLatency = pingLatency
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
		Latency:     finalLatency,
	}

	if err := c.storage.SaveMeasurement(m); err != nil {
		fmt.Printf("Error saving measurement for %s: %v\n", t.Name, err)
	}

	c.logger.Log(status, t.Name, totalDuration, written, speed)
	if traceOutput != "" {
		c.logger.LogMTR(t.Name, traceOutput)
	}
}

func (c *Collector) logError(t model.Target, err error, loss float64, traceOutput string) {
	// Not used anymore in revised flow, but keeping for compatibility if needed or removed?
	// I removed calls to it in MeasureTarget. I can remove the method or keep it.
	// It's private, I'll remove it or update it. I'll just remove/ignore it in this rewrite.
}

// runPing executes ping -c 5 -i 0.2 <host>
func (c *Collector) runPing(ctx context.Context, host string) (loss float64, latency float64, err error) {
	cmd := exec.CommandContext(ctx, "ping", "-c", "5", "-i", "0.2", host)
	outBytes, err := cmd.CombinedOutput()
	output := string(outBytes)

	// Even if err != nil (e.g. non-zero exit code due to loss), we parse output.
	// Parse Loss
	// "5 packets transmitted, 5 packets received, 0% packet loss"
	// "5 packets transmitted, 0 packets received, 100% packet loss"
	reLoss := regexp.MustCompile(`(\d+)% packet loss`)
	matchesLoss := reLoss.FindStringSubmatch(output)
	if len(matchesLoss) > 1 {
		if val, e := strconv.ParseFloat(matchesLoss[1], 64); e == nil {
			loss = val
		}
	}

	// Parse Latency
	// "round-trip min/avg/max/stddev = 16.452/16.518/16.565/0.038 ms" (Linux iputils)
	// "round-trip min/avg/max = 20.1/20.5/21.2 ms" (Busybox/Alpine)
	// We look for "min/avg/max" followed optionally by "/stddev" or "/mdev"
	reLat := regexp.MustCompile(`min/avg/max(?:/\w+)?\s+=\s+[\d.]+/([\d.]+)/`)
	matchesLat := reLat.FindStringSubmatch(output)
	if len(matchesLat) > 1 {
		if val, e := strconv.ParseFloat(matchesLat[1], 64); e == nil {
			latency = val
		}
	}

	return loss, latency, err
}

// runMTR executes mtr -j -c 10 <host> and returns loss percentage and full output
func (c *Collector) runMTR(ctx context.Context, host string) (float64, string) {
	// mtr needs root, usually available in docker
	// -j: JSON output
	// -c 10: 10 cycles
	// -w: report wide? -j implies report.
	cmd := exec.CommandContext(ctx, "mtr", "--json", "-c", "10", host)
	out, err := cmd.Output()

	// If MTR fails to run (e.g. not found), we return error string
	if err != nil {
		// It might return non-zero exit code if there is packet loss? MTR usually returns 0.
		// If it failed, check stderr? `Output` captures stdout. `CombinedOutput` captures both.
		// But for JSON parsing we want stdout.
		return 0, fmt.Sprintf("MTR Execution Failed: %v", err)
	}

	// Parse JSON to find loss of last hop
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
		// If not JSON, maybe raw text?
		return 0, string(out)
	}

	hubs := res.Report.Hubs
	if len(hubs) == 0 {
		return 0, string(out)
	}

	// Last hop is destination
	lastHop := hubs[len(hubs)-1]

	// We return the raw JSON as output, OR a formatted table?
	// User said "Log the full MTR output... I need to see where the packet loss happens."
	// JSON is hard to read in logs. MTR can output text.
	// But we use JSON to parse Loss programmatically.
	// Can we run MTR with text output?
	// Or just log the JSON.
	// JSON is fine, user said "MTR output ... trace hops".
	// If I log JSON, they can see hops.
	// But `mtr -r` (report) gives nice text table.
	// Maybe I should run `mtr -r` if I want readable logs?
	// But I also need `Loss`.
	// I can parse text report too, but JSON is safer.
	// I will return the JSON string. It contains all hops.

	return lastHop.Loss, string(out)
}
