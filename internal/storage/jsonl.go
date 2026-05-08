package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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

// GetMeasurements reads the file backwards and returns the last limit measurements for the target.
func (s *JSONLStorage) GetMeasurements(targetName string, limit int) ([]model.Measurement, error) {
	if limit <= 0 {
		return []model.Measurement{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(s.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Measurement{}, nil
		}
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	if size == 0 {
		return []model.Measurement{}, nil
	}

	var measurements []model.Measurement

	const chunkSize = 64 * 1024          // 64KB
	const maxLineSize = 10 * 1024 * 1024 // 10MB limit for safety to prevent huge allocations

	buffer := make([]byte, chunkSize)
	var leftover []byte // Holds data for the current line being assembled
	var pos int64 = size

	for pos > 0 && len(measurements) < limit {
		move := int64(chunkSize)
		if pos < move {
			move = pos
	targetBytes := []byte(`"target":"` + targetName + `"`)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Fast-path filter: skip lines that definitely don't belong to this target
		if !bytes.Contains(line, targetBytes) {
			continue
		}

		var m model.Measurement
		if err := json.Unmarshal(line, &m); err != nil {
			continue // skip malformed lines
		}
		pos -= move

		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			break
		}

		n, err := f.Read(buffer[:move])
		if err != nil && err != io.EOF {
			break
		}

		chunk := buffer[:n]

		// Process the chunk from end to beginning
		for {
			idx := bytes.LastIndexByte(chunk, '\n')
			if idx == -1 {
				// No newline found, prepend the whole chunk to leftover
				// Check for max line size
				if len(leftover)+len(chunk) > maxLineSize {
					// Skip this malformed/huge line, reset leftover
					leftover = nil
				} else {
					// prepend
					newLeftover := make([]byte, len(chunk)+len(leftover))
					copy(newLeftover, chunk)
					copy(newLeftover[len(chunk):], leftover)
					leftover = newLeftover
				}
				break
			}

			// Found a newline.
			linePart := chunk[idx+1:]

			// Assemble the full line
			var fullLine []byte
			if len(leftover) > 0 {
				if len(linePart)+len(leftover) <= maxLineSize {
					fullLine = make([]byte, len(linePart)+len(leftover))
					copy(fullLine, linePart)
					copy(fullLine[len(linePart):], leftover)
				}
				leftover = nil // Reset for next line
			} else {
				fullLine = linePart
			}

			// Process the full line
			if len(fullLine) > 0 {
				var m model.Measurement
				if err := json.Unmarshal(fullLine, &m); err == nil {
					if m.Target == targetName {
						measurements = append(measurements, m)
						if len(measurements) >= limit {
							break
						}
					}
				}
			}

			// Move chunk to the part before the newline
			chunk = chunk[:idx]
		}
	}

	// Process the very first line of the file (if we reached pos == 0 and there's leftover data)
	if pos == 0 && len(leftover) > 0 && len(measurements) < limit {
		var m model.Measurement
		if err := json.Unmarshal(leftover, &m); err == nil {
			if m.Target == targetName {
				measurements = append(measurements, m)
			}
		}
	}

	return measurements, nil
}

// GetMeasurementsWithRange implements the interface for JSONL but ignores aggregation logic for simplicity in this fallback.
func (s *JSONLStorage) GetMeasurementsWithRange(targetName string, rangeStart string, limit int, aggregate bool, window string) ([]model.Measurement, error) {
	// Fallback to standard get, ignoring range parsing for now as JSONL is secondary/fallback
	return s.GetMeasurements(targetName, limit)
}

func (s *JSONLStorage) Close() error {
	return s.file.Close()
}
