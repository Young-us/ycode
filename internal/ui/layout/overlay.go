package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

// PlaceOverlay places fg on top of bg at position (x, y)
// If shadow is true, adds a drop shadow effect behind the foreground
func PlaceOverlay(
	x, y int,
	fg, bg string,
	shadow bool,
) string {
	fgLines, fgWidth := getLines(fg)
	bgLines, bgWidth := getLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	// Add shadow effect if requested
	if shadow {
		var shadowBg strings.Builder
		shadowChar := " "  // Use space instead of Unicode for Windows compatibility
		bgChar := " "

		for i := 0; i <= fgHeight; i++ {
			if i == 0 {
				shadowBg.WriteString(bgChar)
				shadowBg.WriteString(strings.Repeat(bgChar, fgWidth))
				shadowBg.WriteString("\n")
			} else {
				shadowBg.WriteString(bgChar)
				shadowBg.WriteString(strings.Repeat(shadowChar, fgWidth))
				shadowBg.WriteString("\n")
			}
		}

		fg = PlaceOverlay(0, 0, fg, shadowBg.String(), false)
		fgLines, fgWidth = getLines(fg)
		fgHeight = len(fgLines)
	}

	// If foreground is larger than background, just return foreground
	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		return fg
	}

	// Clamp position to valid range
	x = clamp(x, 0, bgWidth-fgWidth)
	y = clamp(y, 0, bgHeight-fgHeight)

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}

		// Lines before or after the overlay region
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		// Lines within the overlay region
		pos := 0

		// Write left part of background line
		if x > 0 {
			left := truncate.String(bgLine, uint(x))
			pos = lipgloss.Width(left)
			b.WriteString(left)
			if pos < x {
				b.WriteString(strings.Repeat(" ", x-pos))
				pos = x
			}
		}

		// Write foreground line
		fgLine := fgLines[i-y]
		b.WriteString(fgLine)
		pos += lipgloss.Width(fgLine)

		// Write right part of background line
		right := cutLeft(bgLine, pos)
		rightWidth := lipgloss.Width(right)
		if rightWidth <= bgWidth-pos {
			b.WriteString(strings.Repeat(" ", bgWidth-rightWidth-pos))
		}

		b.WriteString(right)
	}

	return b.String()
}

// getLines splits a string into lines and returns the widest line width
func getLines(s string) (lines []string, widest int) {
	lines = strings.Split(s, "\n")

	for _, l := range lines {
		w := lipgloss.Width(l)
		if widest < w {
			widest = w
		}
	}

	return lines, widest
}

// cutLeft cuts printable characters from the left up to cutWidth
func cutLeft(s string, cutWidth int) string {
	if cutWidth <= 0 {
		return s
	}

	totalWidth := lipgloss.Width(s)
	if cutWidth >= totalWidth {
		return ""
	}

	// Use ansi-compliant truncation: remove cutWidth from the left
	// by iterating through the string and tracking visual width
	runes := []rune(s)
	visualPos := 0

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check for ANSI escape sequence
		if r == '\x1b' {
			// Skip the entire escape sequence
			for i++; i < len(runes); i++ {
				// ANSI sequences end with a letter
				if (runes[i] >= 'A' && runes[i] <= 'Z') || (runes[i] >= 'a' && runes[i] <= 'z') {
					break
				}
			}
			continue
		}

		// Get visual width of this character
		charWidth := lipgloss.Width(string(r))
		if visualPos >= cutWidth {
			return string(runes[i:])
		}
		visualPos += charWidth
	}

	return ""
}

// clamp restricts value to [min, max] range
func clamp(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}
