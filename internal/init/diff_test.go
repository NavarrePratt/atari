package initcmd

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_Identical(t *testing.T) {
	content := "line1\nline2\nline3"
	diff := UnifiedDiff("old.txt", "new.txt", content, content)
	if diff != "" {
		t.Errorf("expected empty diff for identical content, got: %s", diff)
	}
}

func TestUnifiedDiff_Empty(t *testing.T) {
	diff := UnifiedDiff("old.txt", "new.txt", "", "")
	if diff != "" {
		t.Errorf("expected empty diff for empty content, got: %s", diff)
	}
}

func TestUnifiedDiff_AddLine(t *testing.T) {
	old := "line1\nline2"
	new := "line1\nline2\nline3"
	diff := UnifiedDiff("old.txt", "new.txt", old, new)

	if !strings.Contains(diff, "--- old.txt") {
		t.Error("diff should contain old file header")
	}
	if !strings.Contains(diff, "+++ new.txt") {
		t.Error("diff should contain new file header")
	}
	if !strings.Contains(diff, "+line3") {
		t.Error("diff should show added line")
	}
}

func TestUnifiedDiff_RemoveLine(t *testing.T) {
	old := "line1\nline2\nline3"
	new := "line1\nline3"
	diff := UnifiedDiff("old.txt", "new.txt", old, new)

	if !strings.Contains(diff, "-line2") {
		t.Error("diff should show removed line")
	}
}

func TestUnifiedDiff_ModifyLine(t *testing.T) {
	old := "line1\nold line\nline3"
	new := "line1\nnew line\nline3"
	diff := UnifiedDiff("old.txt", "new.txt", old, new)

	if !strings.Contains(diff, "-old line") {
		t.Error("diff should show removed old line")
	}
	if !strings.Contains(diff, "+new line") {
		t.Error("diff should show added new line")
	}
}

func TestUnifiedDiff_HunkHeader(t *testing.T) {
	old := "line1\nline2"
	new := "line1\nline2\nline3"
	diff := UnifiedDiff("old.txt", "new.txt", old, new)

	if !strings.Contains(diff, "@@") {
		t.Error("diff should contain hunk header")
	}
}

func TestUnifiedDiff_ContextLines(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5"
	new := "line1\nline2\nmodified\nline4\nline5"
	diff := UnifiedDiff("old.txt", "new.txt", old, new)

	// Should include context lines around the change
	if !strings.Contains(diff, " line2") {
		t.Error("diff should include context line before change")
	}
	if !strings.Contains(diff, " line4") {
		t.Error("diff should include context line after change")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"a\nb", 2},
		{"a\nb\nc", 3},
	}

	for _, tc := range tests {
		lines := splitLines(tc.input)
		if len(lines) != tc.expected {
			t.Errorf("splitLines(%q): expected %d lines, got %d", tc.input, tc.expected, len(lines))
		}
	}
}

func TestComputeEdits_NoChanges(t *testing.T) {
	old := []string{"a", "b", "c"}
	new := []string{"a", "b", "c"}
	edits := computeEdits(old, new)

	for _, e := range edits {
		if e.op != editEqual {
			t.Error("expected all edits to be equal for identical content")
		}
	}
}

func TestComputeEdits_AllNew(t *testing.T) {
	old := []string{}
	new := []string{"a", "b"}
	edits := computeEdits(old, new)

	insertCount := 0
	for _, e := range edits {
		if e.op == editInsert {
			insertCount++
		}
	}
	if insertCount != 2 {
		t.Errorf("expected 2 inserts, got %d", insertCount)
	}
}

func TestComputeEdits_AllDeleted(t *testing.T) {
	old := []string{"a", "b"}
	new := []string{}
	edits := computeEdits(old, new)

	deleteCount := 0
	for _, e := range edits {
		if e.op == editDelete {
			deleteCount++
		}
	}
	if deleteCount != 2 {
		t.Errorf("expected 2 deletes, got %d", deleteCount)
	}
}
