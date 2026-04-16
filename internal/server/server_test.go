package server

import (
	"bufio"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
