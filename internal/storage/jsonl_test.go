package storage

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jules/http-monitor/internal/model"
)

func BenchmarkGetMeasurements(b *testing.B) {
	tmpfile, err := os.CreateTemp("", "benchmark-*.jsonl")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	targets := []string{"Google", "Cloudflare", "GitHub", "Internal"}
	for i := 0; i < 10000; i++ {
		m := model.Measurement{
			Timestamp: time.Now(),
			Target:    targets[i%len(targets)],
			Duration:  1.5,
			Size:      1024,
			Speed:     1024 / 1.5,
			Status:    "OK",
			Latency:   50.0,
		}
		data, _ := json.Marshal(m)
		tmpfile.Write(data)
		tmpfile.Write([]byte("\n"))
	}
	tmpfile.Close()

	s, err := NewJSONLStorage(tmpfile.Name())
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetMeasurements("GitHub", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestJSONLStorage_GetMeasurements(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	s, err := NewJSONLStorage(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Add measurements
	targets := []string{"Google", "GitHub"}
	for i := 0; i < 10; i++ {
		m := model.Measurement{
			Timestamp: time.Now(),
			Target:    targets[i%2],
			Duration:  float64(i),
		}
		if err := s.SaveMeasurement(m); err != nil {
			t.Fatal(err)
		}
	}

	// Read GitHub
	measurements, err := s.GetMeasurements("GitHub", 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(measurements) != 5 {
		t.Fatalf("expected 5 measurements, got %d", len(measurements))
	}

	// They should be in reverse order, meaning newest first.
	// Since GitHub is placed at odd indices, newest should be i=9 (Duration 9).
	if measurements[0].Duration != 9.0 {
		t.Fatalf("expected first duration to be 9.0, got %f", measurements[0].Duration)
	}
}
