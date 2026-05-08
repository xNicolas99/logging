package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jules/http-monitor/internal/config"
)

func TestHandleTargets_Validation(t *testing.T) {
	s := &Server{
		cfg: &config.Config{},
	}

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
	}{
		{
			name: "ValidTarget",
			payload: map[string]interface{}{
				"name":     "Google-DNS",
				"url":      "https://8.8.8.8",
				"interval": 60,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "InvalidName_XSS",
			payload: map[string]interface{}{
				"name": "<script>alert(1)</script>",
				"url":  "https://example.com",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "InvalidURL_Scheme",
			payload: map[string]interface{}{
				"name": "LocalFile",
				"url":  "file:///etc/passwd",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "InvalidURL_Relative",
			payload: map[string]interface{}{
				"name": "Relative",
				"url":  "/api/data",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "NegativeInterval",
			payload: map[string]interface{}{
				"name":     "Negative",
				"url":      "https://example.com",
				"interval": -1,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/targets", bytes.NewBuffer(body))
			rr := httptest.NewRecorder()

			// We need a mock monitor because handleTargets calls s.monitor.AddTarget
			// For this test, we only care about validation before AddTarget is called.
			// However, handleTargets as implemented will call AddTarget if validation passes.
			// Let's use a real monitor but point it to a temp config.
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")
			cfg := &config.Config{}
			s.cfg = cfg
			s.monitor = &monitorMock{} // Use a mock to avoid full initialization

			s.handleTargets(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

type monitorMock struct{}

func (m *monitorMock) AddTarget(name, url string, interval int) error { return nil }
func (m *monitorMock) DeleteTarget(name string) error               { return nil }
func (m *monitorMock) RunNow()                                       {}
func (m *monitorMock) SetInterval(minutes int) error                { return nil }

func TestHandleLogs(t *testing.T) {
	tests := []struct {
		name      string
		lineCount int
	}{
		{"FewLines", 10},
		{"Exactly100Lines", 100},
		{"ManyLines", 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "test.log")
			f, err := os.Create(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= tt.lineCount; i++ {
				fmt.Fprintf(f, "Log line %d\n", i)
			}
			f.Close()

			file, err := os.Open(logPath)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			rr := httptest.NewRecorder()
			err = tailLog(file, rr, 100)
			if err != nil {
				t.Errorf("tailLog failed: %v", err)
			}

			// Verify the number of lines
			scanner := bufio.NewScanner(rr.Body)
			lines := 0
			for scanner.Scan() {
				lines++
			}
			expectedLines := tt.lineCount
			if expectedLines > 100 {
				expectedLines = 100
			}
			if lines != expectedLines {
				t.Errorf("expected %d lines, got %d", expectedLines, lines)
			}
		})
	}
}

func BenchmarkHandleLogs(b *testing.B) {
	tmpDir := b.TempDir()
	logPath := filepath.Join(tmpDir, "bench.log")
	f, err := os.Create(logPath)
	if err != nil {
		b.Fatal(err)
	}

	// Create a ~10MB log file
	lineCount := 100000
	line := "Log line %d: Some random log message to fill up space and make the file reasonably large. abcdefghijklmnopqrstuvwxyz\n"
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(f, line, i)
	}
	f.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file, err := os.Open(logPath)
		if err != nil {
			b.Fatal(err)
		}
		rr := httptest.NewRecorder()
		err = tailLog(file, rr, 100)
		if err != nil {
			b.Fatal(err)
		}
		file.Close()
	}
}
