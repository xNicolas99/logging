package storage

import (
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
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetMeasurements(targetName, 100)
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
	}
}
