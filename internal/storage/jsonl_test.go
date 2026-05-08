package storage

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jules/http-monitor/internal/model"
)

func BenchmarkJSONLStorage_GetMeasurements(b *testing.B) {
	// Setup a temporary JSONL file with many entries
	tmpFile, err := os.CreateTemp("", "benchmark_jsonl_*.jsonl")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	s, err := NewJSONLStorage(tmpFile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()

	// Write 10000 measurements
	targetName := "target-A"
	for i := 0; i < 10000; i++ {
		tName := "target-B"
		if i%10 == 0 {
			tName = targetName // 1000 matching targets
		}
		m := model.Measurement{
			Timestamp: time.Now(),
			Target:    tName,
			Duration:  1.2,
			Size:      1024,
			Speed:     500.5,
			Status:    "OK",
		}
		s.SaveMeasurement(m)
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
		_, err := s.GetMeasurements(targetName, 100)
		_, err := s.GetMeasurements("GitHub", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestJSONLStorage_GetMeasurements(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_jsonl_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	s, err := NewJSONLStorage(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Write 5 measurements for target-A and 5 for target-B
	for i := 0; i < 10; i++ {
		tName := "target-B"
		if i%2 == 0 {
			tName = "target-A"
		}
		m := model.Measurement{
			Timestamp: time.Now(),
			Target:    tName,
			Size:      int64(i),
		}
		s.SaveMeasurement(m)
	}

	// target-A has sizes 0, 2, 4, 6, 8
	// The most recent 3 sizes should be 8, 6, 4
	measurements, err := s.GetMeasurements("target-A", 3)
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

	if len(measurements) != 3 {
		t.Fatalf("Expected 3 measurements, got %d", len(measurements))
	}

	if measurements[0].Size != 8 {
		t.Errorf("Expected first measurement size 8, got %d", measurements[0].Size)
	}
	if measurements[1].Size != 6 {
		t.Errorf("Expected second measurement size 6, got %d", measurements[1].Size)
	}
	if measurements[2].Size != 4 {
		t.Errorf("Expected third measurement size 4, got %d", measurements[2].Size)
	if len(measurements) != 5 {
		t.Fatalf("expected 5 measurements, got %d", len(measurements))
	}

	// They should be in reverse order, meaning newest first.
	// Since GitHub is placed at odd indices, newest should be i=9 (Duration 9).
	if measurements[0].Duration != 9.0 {
		t.Fatalf("expected first duration to be 9.0, got %f", measurements[0].Duration)
	}
}
