package initcmd

import (
	"fmt"
	"strings"
)

// UnifiedDiff generates a unified diff between two strings.
// Returns empty string if contents are identical.
func UnifiedDiff(oldName, newName, oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Compute LCS and generate edit script
	edits := computeEdits(oldLines, newLines)
	if len(edits) == 0 {
		return ""
	}

	// Generate unified diff output
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n", oldName)
	fmt.Fprintf(&out, "+++ %s\n", newName)

	// Group edits into hunks with context
	hunks := groupIntoHunks(edits, oldLines, newLines, 3)
	for _, hunk := range hunks {
		out.WriteString(hunk)
	}

	return out.String()
}

// editOp represents a diff operation
type editOp int

const (
	editEqual editOp = iota
	editDelete
	editInsert
)

// edit represents a single edit operation
type edit struct {
	op       editOp
	oldIndex int // line index in old content (-1 for insert)
	newIndex int // line index in new content (-1 for delete)
}

// splitLines splits content into lines, preserving empty trailing line info
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// computeEdits uses a simple LCS-based algorithm to compute edits
func computeEdits(oldLines, newLines []string) []edit {
	// Build LCS table
	m, n := len(oldLines), len(newLines)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Backtrack to find edits
	var edits []edit
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			edits = append(edits, edit{op: editEqual, oldIndex: i - 1, newIndex: j - 1})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			edits = append(edits, edit{op: editInsert, oldIndex: -1, newIndex: j - 1})
			j--
		} else {
			edits = append(edits, edit{op: editDelete, oldIndex: i - 1, newIndex: -1})
			i--
		}
	}

	// Reverse to get chronological order
	for left, right := 0, len(edits)-1; left < right; left, right = left+1, right-1 {
		edits[left], edits[right] = edits[right], edits[left]
	}

	return edits
}

// groupIntoHunks groups edits into unified diff hunks with context lines
func groupIntoHunks(edits []edit, oldLines, newLines []string, contextLines int) []string {
	var hunks []string

	// Find ranges of changes
	type changeRange struct {
		start, end int // indices into edits slice
	}

	var ranges []changeRange
	inChange := false
	var currentRange changeRange

	for i, e := range edits {
		isChange := e.op != editEqual
		if isChange && !inChange {
			currentRange.start = i
			inChange = true
		} else if !isChange && inChange {
			currentRange.end = i
			ranges = append(ranges, currentRange)
			inChange = false
		}
	}
	if inChange {
		currentRange.end = len(edits)
		ranges = append(ranges, currentRange)
	}

	if len(ranges) == 0 {
		return nil
	}

	// Merge nearby ranges and add context
	for _, r := range ranges {
		// Expand range for context
		start := r.start - contextLines
		if start < 0 {
			start = 0
		}
		end := r.end + contextLines
		if end > len(edits) {
			end = len(edits)
		}

		// Calculate line numbers for hunk header
		oldStart, oldCount := 0, 0
		newStart, newCount := 0, 0
		firstOld, firstNew := true, true

		var lines []string
		for i := start; i < end; i++ {
			e := edits[i]
			switch e.op {
			case editEqual:
				if firstOld && e.oldIndex >= 0 {
					oldStart = e.oldIndex + 1
					firstOld = false
				}
				if firstNew && e.newIndex >= 0 {
					newStart = e.newIndex + 1
					firstNew = false
				}
				oldCount++
				newCount++
				lines = append(lines, " "+oldLines[e.oldIndex])
			case editDelete:
				if firstOld {
					oldStart = e.oldIndex + 1
					firstOld = false
				}
				oldCount++
				lines = append(lines, "-"+oldLines[e.oldIndex])
			case editInsert:
				if firstNew {
					newStart = e.newIndex + 1
					firstNew = false
				}
				newCount++
				lines = append(lines, "+"+newLines[e.newIndex])
			}
		}

		// Handle edge case where we start at beginning
		if oldStart == 0 {
			oldStart = 1
		}
		if newStart == 0 {
			newStart = 1
		}

		var hunk strings.Builder
		fmt.Fprintf(&hunk, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, line := range lines {
			hunk.WriteString(line)
			hunk.WriteString("\n")
		}
		hunks = append(hunks, hunk.String())
	}

	return hunks
}
