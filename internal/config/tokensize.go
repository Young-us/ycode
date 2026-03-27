package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseTokenSize parses human-readable token size strings like "128k", "4k", "1M" into integers
// Supported suffixes: k (thousand), M (million), G (billion)
// Examples: "128k" -> 128000, "4k" -> 4000, "1M" -> 1000000
func ParseTokenSize(size string) (int, error) {
	// Trim whitespace
	size = strings.TrimSpace(size)

	// If it's a plain number, parse it directly
	if num, err := strconv.Atoi(size); err == nil {
		return num, nil
	}

	// Parse with suffix
	size = strings.ToLower(size)

	var multiplier int
	var numStr string

	if strings.HasSuffix(size, "k") {
		multiplier = 1000
		numStr = strings.TrimSuffix(size, "k")
	} else if strings.HasSuffix(size, "m") {
		multiplier = 1000000
		numStr = strings.TrimSuffix(size, "m")
	} else if strings.HasSuffix(size, "g") {
		multiplier = 1000000000
		numStr = strings.TrimSuffix(size, "g")
	} else {
		return 0, fmt.Errorf("invalid token size format: %s (supported: 128k, 4k, 1M, 1G)", size)
	}

	// Parse the number part
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in token size: %s", numStr)
	}

	// Calculate result
	result := int(num * float64(multiplier))

	// Validate reasonable bounds
	if result < 1 {
		return 0, fmt.Errorf("token size too small: %d", result)
	}
	if result > 1000000000 { // 1B tokens seems unreasonable
		return 0, fmt.Errorf("token size too large: %d", result)
	}

	return result, nil
}

// FormatTokenSize formats an integer token count into human-readable format
func FormatTokenSize(tokens int) string {
	if tokens >= 1000000000 {
		return fmt.Sprintf("%.1fG", float64(tokens)/1000000000)
	}
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return strconv.Itoa(tokens)
}
