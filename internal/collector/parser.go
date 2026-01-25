package collector

import (
	"strconv"
	"strings"
)

// parseMtrLoss extracts the packet loss percentage from the last hop of an MTR report.
func parseMtrLoss(output string) float64 {
	lines := strings.Split(output, "\n")
	var lastLine string
	// Find last valid hop line (starts with number)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// MTR hop lines start with row number like " 1.|--" or "1."
		// Checking if it starts with digit is a good heuristic.
		if len(line) > 0 && (line[0] >= '0' && line[0] <= '9') {
			lastLine = line
			break
		}
	}

	if lastLine == "" {
		return 0.0
	}

	fields := strings.Fields(lastLine)
	// Expected: "N.|--", "Host", "Loss%", "Snt", ...
	// Search for field with '%'
	for _, f := range fields {
		if strings.Contains(f, "%") {
			valStr := strings.TrimRight(f, "%")
			if val, err := strconv.ParseFloat(valStr, 64); err == nil {
				return val
			}
		}
	}

	return 0.0
}
