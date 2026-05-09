package collector

import (
	"errors"
	"testing"
)

func TestDetermineStatusAndMTR(t *testing.T) {
	tests := []struct {
		name        string
		isSpeedTest bool
		dlErr       error
		statusCode  int
		speed       float64
		threshold   float64
		pingLoss    float64
		pingLatency float64
		wantStatus  string
		wantRunMTR  bool
	}{
		// Speed Test Cases
		{
			name:        "SpeedTest Success",
			isSpeedTest: true,
			dlErr:       nil,
			statusCode:  200,
			speed:       1000.0,
			threshold:   500.0,
			wantStatus:  "OK",
			wantRunMTR:  false,
		},
		{
			name:        "SpeedTest Slow",
			isSpeedTest: true,
			dlErr:       nil,
			statusCode:  200,
			speed:       400.0,
			threshold:   500.0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},
		{
			name:        "SpeedTest HTTP Error",
			isSpeedTest: true,
			dlErr:       errors.New("connection reset"),
			statusCode:  0,
			speed:       0,
			threshold:   500.0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},
		{
			name:        "SpeedTest Non-200 Status",
			isSpeedTest: true,
			dlErr:       nil,
			statusCode:  500,
			speed:       100.0,
			threshold:   500.0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},

		// Web Check Cases
		{
			name:        "WebCheck Success",
			isSpeedTest: false,
			dlErr:       nil,
			statusCode:  200,
			pingLoss:    0,
			pingLatency: 20.0,
			wantStatus:  "OK",
			wantRunMTR:  false,
		},
		{
			name:        "WebCheck High Latency",
			isSpeedTest: false,
			dlErr:       nil,
			statusCode:  200,
			pingLoss:    0,
			pingLatency: 150.0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},
		{
			name:        "WebCheck Packet Loss",
			isSpeedTest: false,
			dlErr:       nil,
			statusCode:  200,
			pingLoss:    10.0,
			pingLatency: 20.0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},
		{
			name:        "WebCheck HTTP Error",
			isSpeedTest: false,
			dlErr:       errors.New("timeout"),
			statusCode:  0,
			pingLoss:    0,
			pingLatency: 0,
			wantStatus:  "ALERT",
			wantRunMTR:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotRunMTR := determineStatusAndMTR(tt.isSpeedTest, tt.dlErr, tt.statusCode, tt.speed, tt.threshold, tt.pingLoss, tt.pingLatency)
			if gotStatus != tt.wantStatus {
				t.Errorf("determineStatusAndMTR() gotStatus = %v, want %v", gotStatus, tt.wantStatus)
			}
			if gotRunMTR != tt.wantRunMTR {
				t.Errorf("determineStatusAndMTR() gotRunMTR = %v, want %v", gotRunMTR, tt.wantRunMTR)
			}
		})
	}
}

func TestIsValidHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"google.com", true},
		{"1.1.1.1", true},
		{"my-host", true},
		{"-host", false},
		{"host;ls", false},
		{"host$(ls)", false},
		{"host.com/path", false},
	}

	for _, tt := range tests {
		if got := isValidHost(tt.host); got != tt.want {
			t.Errorf("isValidHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}
