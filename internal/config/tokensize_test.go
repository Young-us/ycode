package config

import (
	"testing"
)

func TestParseTokenSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		hasError bool
	}{
		{
			name:     "plain number",
			input:    "4096",
			expected: 4096,
			hasError: false,
		},
		{
			name:     "kilobytes",
			input:    "4k",
			expected: 4000,
			hasError: false,
		},
		{
			name:     "megabytes",
			input:    "1M",
			expected: 1000000,
			hasError: false,
		},
		{
			name:     "gigabytes",
			input:    "1G",
			expected: 1000000000,
			hasError: false,
		},
		{
			name:     "with whitespace",
			input:    "  128k  ",
			expected: 128000,
			hasError: false,
		},
		{
			name:     "decimal kilobytes",
			input:    "4.5k",
			expected: 4500,
			hasError: false,
		},
		{
			name:     "invalid format",
			input:    "128x",
			hasError: true,
		},
		{
			name:     "invalid number",
			input:    "abcM",
			hasError: true,
		},
		{
			name:     "too large",
			input:    "2G",
			hasError: true,
		},
		{
			name:     "zero",
			input:    "0k",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTokenSize(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error for input %q, got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("for input %q, expected %d, got %d", tt.input, tt.expected, result)
				}
			}
		})
	}
}

func TestFormatTokenSize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "small number",
			input:    500,
			expected: "500",
		},
		{
			name:     "kilobytes",
			input:    4000,
			expected: "4.0k",
		},
		{
			name:     "megabytes",
			input:    1000000,
			expected: "1.0M",
		},
		{
			name:     "gigabytes",
			input:    1000000000,
			expected: "1.0G",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTokenSize(tt.input)
			if result != tt.expected {
				t.Errorf("for input %d, expected %q, got %q", tt.input, tt.expected, result)
			}
		})
	}
}
