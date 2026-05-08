package collector

import (
	"testing"
)

func TestParseMtrLoss(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected float64
	}{
		{
			name: "Standard MTR output",
			output: `Start: 2024-05-24T10:00:00+0000
HOST: my-computer             Loss%   Snt   Last   Avg  Best  Wrst StDev
  1.|-- 192.168.1.1            0.0%    10    0.3   0.3   0.2   0.5   0.1
  2.|-- 10.0.0.1               0.0%    10    1.2   1.5   1.1   2.3   0.4
  3.|-- 8.8.8.8               10.0%    10   15.4  15.6  15.2  16.3   0.3`,
			expected: 10.0,
		},
		{
			name: "Single hop MTR output",
			output: `HOST: localhost               Loss%   Snt   Last   Avg  Best  Wrst StDev
  1.|-- 127.0.0.1              0.0%    10    0.1   0.1   0.1   0.1   0.0`,
			expected: 0.0,
		},
		{
			name: "Trailing whitespace and empty lines",
			output: `  1.|-- 1.1.1.1               5.5%    10    1.0   1.0   1.0   1.0   0.0


`,
			expected: 5.5,
		},
		{
			name: "No valid hop lines",
			output: `Start: 2024-05-24T10:00:00+0000
Wait for output...`,
			expected: 0.0,
		},
		{
			name: "100% loss",
			output: `  1.|-- 192.168.1.1            0.0%    10    0.3   0.3   0.2   0.5   0.1
  2.|-- ???                  100.0%    10    0.0   0.0   0.0   0.0   0.0`,
			expected: 100.0,
		},
		{
			name: "Invalid percentage format (no digit)",
			output: `  1.|-- 1.1.1.1               %%      10    1.0   1.0   1.0   1.0   0.0`,
			expected: 0.0,
		},
		{
			name: "Empty input",
			output:   "",
			expected: 0.0,
		},
		{
			name: "MTR output with different field order",
			output: `  1.|-- 8.8.8.8    10   20.5%   0.3   0.3   0.2   0.5   0.1`,
			expected: 20.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := parseMtrLoss(tt.output)
			if actual != tt.expected {
				t.Errorf("parseMtrLoss() = %v, want %v", actual, tt.expected)
			}
		})
	}
}
