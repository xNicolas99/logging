package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTailLog(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		maxLines  int
		wantLines []string
	}{
		{
			name:      "EmptyFile",
			content:   "",
			maxLines:  10,
			wantLines: []string{},
		},
		{
			name:      "MaxLinesZero",
			content:   "line1\nline2\n",
			maxLines:  0,
			wantLines: []string{},
		},
		{
			name:      "FewerLinesThanMax",
			content:   "line1\nline2\n",
			maxLines:  5,
			wantLines: []string{"line1", "line2"},
		},
		{
			name:      "ExactMaxLines",
			content:   "line1\nline2\nline3\n",
			maxLines:  3,
			wantLines: []string{"line1", "line2", "line3"},
		},
		{
			name:      "MoreLinesThanMax",
			content:   "line1\nline2\nline3\nline4\n",
			maxLines:  2,
			wantLines: []string{"line3", "line4"},
		},
		{
			name:      "NoTrailingNewline",
			content:   "line1\nline2",
			maxLines:  1,
			wantLines: []string{"line2"},
		},
		{
			name:      "MultipleTrailingNewlines",
			content:   "line1\nline2\n\n",
			maxLines:  2,
			wantLines: []string{"line2", ""},
		},
		{
			name:      "LongLines",
			content:   strings.Repeat("a", 5000) + "\n" + strings.Repeat("b", 5000) + "\n",
			maxLines:  1,
			wantLines: []string{strings.Repeat("b", 5000)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logPath := filepath.Join(tmpDir, "test.log")
			err := os.WriteFile(logPath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatal(err)
			}

			file, err := os.Open(logPath)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			var buf bytes.Buffer
			err = tailLog(file, &buf, tt.maxLines)
			if err != nil {
				t.Errorf("tailLog failed: %v", err)
			}

			var gotLines []string
			scanner := bufio.NewScanner(&buf)
			for scanner.Scan() {
				gotLines = append(gotLines, scanner.Text())
			}

			if len(gotLines) != len(tt.wantLines) {
				t.Fatalf("expected %d lines, got %d", len(tt.wantLines), len(gotLines))
			}
			for i := range gotLines {
				if gotLines[i] != tt.wantLines[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.wantLines[i], gotLines[i])
				}
			}
		})
	}
}

func TestHandleLogs_Migrated(t *testing.T) {
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

			var buf bytes.Buffer
			err = tailLog(file, &buf, 100)
			if err != nil {
				t.Errorf("tailLog failed: %v", err)
			}

			// Verify the number of lines
			scanner := bufio.NewScanner(&buf)
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
		var buf bytes.Buffer
		err = tailLog(file, &buf, 100)
		if err != nil {
			b.Fatal(err)
		}
		file.Close()
	}
}

func TestTailLog_Errors(t *testing.T) {
	t.Run("InvalidFile", func(t *testing.T) {
		err := tailLog(nil, io.Discard, 10)
		if err == nil {
			t.Error("expected error for nil file")
		}
	})
}
