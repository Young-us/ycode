package tools

import (
	"fmt"
	"strings"
)

// DiffOperation represents a single diff operation
type DiffOperation struct {
	Type   string // "add", "delete", "equal"
	OldPos int
	NewPos int
	Lines  []string
}

// DiffResult represents the result of a diff operation
type DiffResult struct {
	OldFile string
	NewFile string
	Ops     []DiffOperation
	Stats   DiffStats
}

// DiffStats contains statistics about the diff
type DiffStats struct {
	Additions  int
	Deletions  int
	Unchanged  int
}

// ComputeDiff computes a simple line-based diff between old and new content
func ComputeDiff(oldContent, newContent string) *DiffResult {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	result := &DiffResult{
		OldFile: oldContent,
		NewFile: newContent,
		Ops:     make([]DiffOperation, 0),
	}

	// Simple LCS-based diff
	lcs := computeLCS(oldLines, newLines)

	oldIdx := 0
	newIdx := 0
	lcsIdx := 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if lcsIdx < len(lcs) {
			// Handle deletions (lines in old but not in LCS)
			for oldIdx < len(oldLines) && oldLines[oldIdx] != lcs[lcsIdx] {
				result.Ops = append(result.Ops, DiffOperation{
					Type:   "delete",
					OldPos: oldIdx + 1,
					Lines:  []string{oldLines[oldIdx]},
				})
				result.Stats.Deletions++
				oldIdx++
			}

			// Handle additions (lines in new but not in LCS)
			for newIdx < len(newLines) && newLines[newIdx] != lcs[lcsIdx] {
				result.Ops = append(result.Ops, DiffOperation{
					Type:   "add",
					NewPos: newIdx + 1,
					Lines:  []string{newLines[newIdx]},
				})
				result.Stats.Additions++
				newIdx++
			}

			// Handle equal lines
			if oldIdx < len(oldLines) && newIdx < len(newLines) &&
				oldLines[oldIdx] == lcs[lcsIdx] && newLines[newIdx] == lcs[lcsIdx] {
				result.Ops = append(result.Ops, DiffOperation{
					Type:   "equal",
					OldPos: oldIdx + 1,
					NewPos: newIdx + 1,
					Lines:  []string{oldLines[oldIdx]},
				})
				result.Stats.Unchanged++
				oldIdx++
				newIdx++
				lcsIdx++
			}
		} else {
			// Remaining deletions
			for oldIdx < len(oldLines) {
				result.Ops = append(result.Ops, DiffOperation{
					Type:   "delete",
					OldPos: oldIdx + 1,
					Lines:  []string{oldLines[oldIdx]},
				})
				result.Stats.Deletions++
				oldIdx++
			}
			// Remaining additions
			for newIdx < len(newLines) {
				result.Ops = append(result.Ops, DiffOperation{
					Type:   "add",
					NewPos: newIdx + 1,
					Lines:  []string{newLines[newIdx]},
				})
				result.Stats.Additions++
				newIdx++
			}
		}
	}

	return result
}

// computeLCS computes the Longest Common Subsequence of two string slices
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// Backtrack to find LCS
	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append([]string{a[i-1]}, lcs...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

// FormatDiff formats a diff result for display
func FormatDiff(diff *DiffResult, contextLines int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- Original\n+++ Modified\n"))
	sb.WriteString(fmt.Sprintf("@@ %d additions, %d deletions @@\n\n", diff.Stats.Additions, diff.Stats.Deletions))

	// Group consecutive operations
	i := 0
	for i < len(diff.Ops) {
		op := diff.Ops[i]

		switch op.Type {
		case "add":
			sb.WriteString(fmt.Sprintf("\x1b[32m+ %s\x1b[0m\n", op.Lines[0]))
		case "delete":
			sb.WriteString(fmt.Sprintf("\x1b[31m- %s\x1b[0m\n", op.Lines[0]))
		case "equal":
			// Show context lines in gray
			sb.WriteString(fmt.Sprintf("\x1b[90m  %s\x1b[0m\n", op.Lines[0]))
		}
		i++
	}

	return sb.String()
}

// FormatDiffPlain formats a diff without ANSI colors (for logging)
func FormatDiffPlain(diff *DiffResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- Original\n+++ Modified\n"))
	sb.WriteString(fmt.Sprintf("@@ %d additions, %d deletions @@\n\n", diff.Stats.Additions, diff.Stats.Deletions))

	for _, op := range diff.Ops {
		switch op.Type {
		case "add":
			sb.WriteString(fmt.Sprintf("+ %s\n", op.Lines[0]))
		case "delete":
			sb.WriteString(fmt.Sprintf("- %s\n", op.Lines[0]))
		case "equal":
			sb.WriteString(fmt.Sprintf("  %s\n", op.Lines[0]))
		}
	}

	return sb.String()
}