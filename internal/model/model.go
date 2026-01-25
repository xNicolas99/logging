package model

import "time"

// Target represents a monitoring target configuration.
type Target struct {
	Name      string  `json:"name"`
	URL       string  `json:"url"`
	Threshold float64 `json:"threshold"` // Minimum bytes per second
	Interval  int     `json:"interval"`  // Interval in minutes
}

// Measurement represents a single download test result.
type Measurement struct {
	Timestamp   time.Time `json:"timestamp"`
	Target      string    `json:"target"`
	Duration    float64   `json:"duration"`              // Seconds
	Size        int64     `json:"size"`                  // Bytes
	Speed       float64   `json:"speed"`                 // Bytes per second
	Status      string    `json:"status"`                // "OK" or "ALERT"
	PacketLoss  float64   `json:"packet_loss"`           // Percentage
	TraceOutput string    `json:"trace_output,omitempty"` // Full mtr output
	IsSpeedTest bool      `json:"is_speed_test"`         // True if large file download, False if web check
	Latency     float64   `json:"latency"`               // Latency in milliseconds
}
