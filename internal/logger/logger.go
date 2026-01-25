package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type FileLogger struct {
	mu   sync.Mutex
	file *os.File
}

func NewFileLogger(path string) (*FileLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &FileLogger{file: f}, nil
}

func (l *FileLogger) Log(status, targetName string, duration float64, size int64, speed float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Format: STATUS [timestamp] [target-name] time=<seconds> size=<bytes> speed=<bytes_per_sec> status=STATUS
	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("%s [%s] [%s] time=%.2f size=%d speed=%.2f status=%s\n",
		status, timestamp, targetName, duration, size, speed, status)

	l.file.WriteString(line)
}

func (l *FileLogger) LogMTR(targetName, output string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)
	header := fmt.Sprintf("MTR REPORT [%s] [%s]\n", timestamp, targetName)
	l.file.WriteString(header)
	l.file.WriteString(output)
	l.file.WriteString("\n--------------------------------------------------\n")
}

func (l *FileLogger) Close() {
	l.file.Close()
}
