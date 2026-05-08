package storage

import (
	"os"
	"testing"
	"time"
	"github.com/jules/http-monitor/internal/model"
)

func BenchmarkReversalBaseline(b *testing.B) {
	limit := 100
	count := 10000
	measurements := make([]model.Measurement, count)
	for i := 0; i < count; i++ {
		measurements[i] = model.Measurement{Size: int64(i)}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// Simulation of old logic: reverse then truncate
		for i, j := 0, len(measurements)-1; i < j; i, j = i+1, j-1 {
			measurements[i], measurements[j] = measurements[j], measurements[i]
		}
		_ = measurements[:limit]
		// Reverse back
		for i, j := 0, len(measurements)-1; i < j; i, j = i+1, j-1 {
			measurements[i], measurements[j] = measurements[j], measurements[i]
		}
	}
}

func BenchmarkReversalOptimized(b *testing.B) {
	limit := 100
	count := 10000
	measurements := make([]model.Measurement, count)
	for i := 0; i < count; i++ {
		measurements[i] = model.Measurement{Size: int64(i)}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		var sub []model.Measurement
		if len(measurements) > limit {
			sub = measurements[len(measurements)-limit:]
		} else {
			sub = measurements
		}

		for i, j := 0, len(sub)-1; i < j; i, j = i+1, j-1 {
			sub[i], sub[j] = sub[j], sub[i]
		}
		// Reverse back
		for i, j := 0, len(sub)-1; i < j; i, j = i+1, j-1 {
			sub[i], sub[j] = sub[j], sub[i]
		}
	}
}

func TestGetMeasurementsOrder(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_jsonl_*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	s, err := NewJSONLStorage(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	target := "test-target"
	for i := 1; i <= 10; i++ {
		m := model.Measurement{
			Target:    target,
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Size:      int64(i),
		}
		if err := s.SaveMeasurement(m); err != nil {
			t.Fatal(err)
		}
	}

	limit := 5
	got, err := s.GetMeasurements(target, limit)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != limit {
		t.Errorf("expected %d measurements, got %d", limit, len(got))
	}

	// Should be the LAST 5 measurements, in REVERSE order (latest first)
	// Original indices: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
	// Last 5: 6, 7, 8, 9, 10
	// Reversed: 10, 9, 8, 7, 6
	expectedSizes := []int64{10, 9, 8, 7, 6}
	for i, size := range expectedSizes {
		if got[i].Size != size {
			t.Errorf("at index %d: expected size %d, got %d", i, size, got[i].Size)
		}
	}
}
