package storage

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"

	"github.com/jules/http-monitor/internal/model"
)

// JSONLStorage implements Storage using a JSONL file.
type JSONLStorage struct {
	filepath string
	mu       sync.Mutex
	file     *os.File
}

// NewJSONLStorage initializes the JSONL storage.
func NewJSONLStorage(path string) (*JSONLStorage, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &JSONLStorage{
		filepath: path,
		file:     file,
	}, nil
}

func (s *JSONLStorage) SaveMeasurement(m model.Measurement) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := s.file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// GetMeasurements reads the file and returns the last limit measurements for the target.
// Note: This is a naive implementation that reads the whole file. For large files, seeking from end would be better.
func (s *JSONLStorage) GetMeasurements(targetName string, limit int) ([]model.Measurement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// We need to read from the file. The file handle we have is open for append/write only.
	// So we open a new reader.
	f, err := os.Open(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Measurement{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var measurements []model.Measurement
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m model.Measurement
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			continue // skip malformed lines
		}
		if m.Target == targetName {
			measurements = append(measurements, m)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Reverse to get latest first
	for i, j := 0, len(measurements)-1; i < j; i, j = i+1, j-1 {
		measurements[i], measurements[j] = measurements[j], measurements[i]
	}

	if len(measurements) > limit {
		measurements = measurements[:limit]
	}

	return measurements, nil
}

func (s *JSONLStorage) Close() error {
	return s.file.Close()
}
