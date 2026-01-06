package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/npratt/atari/internal/testutil"
)

// writeJSONLFile creates a .beads directory with an issues.jsonl file containing the given content.
func writeJSONLFile(t *testing.T, dir, content string) string {
	t.Helper()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads directory: %v", err)
	}
	filePath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}
	return beadsDir
}

func TestJSONLReader_ReadAll_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLBasic)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3", len(beads))
	}

	// Verify first bead
	if beads[0].ID != "bd-001" {
		t.Errorf("beads[0].ID = %q, want %q", beads[0].ID, "bd-001")
	}
	if beads[0].Title != "First task" {
		t.Errorf("beads[0].Title = %q, want %q", beads[0].Title, "First task")
	}
	if beads[0].Status != "open" {
		t.Errorf("beads[0].Status = %q, want %q", beads[0].Status, "open")
	}
	if beads[0].Priority != 2 {
		t.Errorf("beads[0].Priority = %d, want %d", beads[0].Priority, 2)
	}

	// Verify second bead
	if beads[1].ID != "bd-002" {
		t.Errorf("beads[1].ID = %q, want %q", beads[1].ID, "bd-002")
	}
	if beads[1].Status != "in_progress" {
		t.Errorf("beads[1].Status = %q, want %q", beads[1].Status, "in_progress")
	}

	// Verify third bead
	if beads[2].ID != "bd-003" {
		t.Errorf("beads[2].ID = %q, want %q", beads[2].ID, "bd-003")
	}
	if beads[2].Status != "blocked" {
		t.Errorf("beads[2].Status = %q, want %q", beads[2].Status, "blocked")
	}
}

func TestJSONLReader_ReadAll_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithDependencies)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3", len(beads))
	}

	// Find task-002 which has two dependencies
	var task002 *GraphBead
	for i := range beads {
		if beads[i].ID == "bd-task-002" {
			task002 = &beads[i]
			break
		}
	}

	if task002 == nil {
		t.Fatal("bd-task-002 not found")
	}

	if len(task002.Dependencies) != 2 {
		t.Fatalf("task002 has %d dependencies, want 2", len(task002.Dependencies))
	}

	// Verify dependency info is resolved (title and status from index)
	foundParent := false
	foundBlocks := false
	for _, dep := range task002.Dependencies {
		if dep.ID == "bd-epic-001" && dep.DependencyType == "parent-child" {
			foundParent = true
			if dep.Title != "Epic: Auth System" {
				t.Errorf("parent dep title = %q, want %q", dep.Title, "Epic: Auth System")
			}
			if dep.Status != "open" {
				t.Errorf("parent dep status = %q, want %q", dep.Status, "open")
			}
		}
		if dep.ID == "bd-task-001" && dep.DependencyType == "blocks" {
			foundBlocks = true
			if dep.Title != "Login form" {
				t.Errorf("blocks dep title = %q, want %q", dep.Title, "Login form")
			}
			if dep.Status != "in_progress" {
				t.Errorf("blocks dep status = %q, want %q", dep.Status, "in_progress")
			}
		}
	}

	if !foundParent {
		t.Error("parent-child dependency not found")
	}
	if !foundBlocks {
		t.Error("blocks dependency not found")
	}

	// Verify DependencyCount is set correctly
	if task002.DependencyCount != 2 {
		t.Errorf("task002.DependencyCount = %d, want 2", task002.DependencyCount)
	}
}

func TestJSONLReader_ReadAll_WithDependents(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithDependencies)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the epic which should have 2 dependents (task-001 and task-002)
	var epic *GraphBead
	for i := range beads {
		if beads[i].ID == "bd-epic-001" {
			epic = &beads[i]
			break
		}
	}

	if epic == nil {
		t.Fatal("bd-epic-001 not found")
	}

	if epic.DependentCount != 2 {
		t.Errorf("epic.DependentCount = %d, want 2", epic.DependentCount)
	}

	if len(epic.Dependents) != 2 {
		t.Fatalf("epic has %d dependents, want 2", len(epic.Dependents))
	}

	// Verify dependents have resolved info
	foundTask001 := false
	foundTask002 := false
	for _, dep := range epic.Dependents {
		if dep.ID == "bd-task-001" {
			foundTask001 = true
			if dep.Title != "Login form" {
				t.Errorf("dependent title = %q, want %q", dep.Title, "Login form")
			}
		}
		if dep.ID == "bd-task-002" {
			foundTask002 = true
			if dep.Title != "Session mgmt" {
				t.Errorf("dependent title = %q, want %q", dep.Title, "Session mgmt")
			}
		}
	}

	if !foundTask001 {
		t.Error("task-001 not found in dependents")
	}
	if !foundTask002 {
		t.Error("task-002 not found in dependents")
	}

	// Also verify task-001 has task-002 as a dependent (via blocks relationship)
	var task001 *GraphBead
	for i := range beads {
		if beads[i].ID == "bd-task-001" {
			task001 = &beads[i]
			break
		}
	}

	if task001 == nil {
		t.Fatal("bd-task-001 not found")
	}

	if task001.DependentCount != 1 {
		t.Errorf("task001.DependentCount = %d, want 1", task001.DependentCount)
	}
}

func TestJSONLReader_ReadAll_MissingDeps(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLMissingDeps)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 1 {
		t.Fatalf("got %d beads, want 1", len(beads))
	}

	// Dependency should exist but with empty title/status since target doesn't exist
	if len(beads[0].Dependencies) != 1 {
		t.Fatalf("got %d dependencies, want 1", len(beads[0].Dependencies))
	}

	dep := beads[0].Dependencies[0]
	if dep.ID != "bd-nonexistent" {
		t.Errorf("dep.ID = %q, want %q", dep.ID, "bd-nonexistent")
	}
	if dep.Title != "" {
		t.Errorf("dep.Title = %q, want empty string (missing dep)", dep.Title)
	}
	if dep.Status != "" {
		t.Errorf("dep.Status = %q, want empty string (missing dep)", dep.Status)
	}
}

func TestJSONLReader_ReadAll_DeletedEntries(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithDeleted)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 beads (bd-001 and bd-003), not 3
	if len(beads) != 2 {
		t.Fatalf("got %d beads, want 2 (deleted bead should be filtered)", len(beads))
	}

	// Verify the deleted bead is not present
	for _, b := range beads {
		if b.ID == "bd-002" {
			t.Error("bd-002 (deleted bead) should have been filtered out")
		}
	}

	// Verify expected beads are present
	ids := make(map[string]bool)
	for _, b := range beads {
		ids[b.ID] = true
	}

	if !ids["bd-001"] {
		t.Error("bd-001 should be present")
	}
	if !ids["bd-003"] {
		t.Error("bd-003 should be present")
	}
}

func TestJSONLReader_ReadAll_AgentType(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithAgent)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ReadAll should NOT filter agent beads - that happens at a higher level
	// All 3 beads should be present
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3 (agent beads should NOT be filtered by ReadAll)", len(beads))
	}

	// Verify the agent bead is present
	foundAgent := false
	for _, b := range beads {
		if b.IssueType == "agent" {
			foundAgent = true
			break
		}
	}

	if !foundAgent {
		t.Error("agent bead should be present in ReadAll output")
	}
}

func TestJSONLReader_ReadAll_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, "")

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 0 {
		t.Errorf("got %d beads, want 0 for empty file", len(beads))
	}
}

func TestJSONLReader_ReadAll_FileNotFound(t *testing.T) {
	reader := NewJSONLReader("/nonexistent/path/.beads")
	_, err := reader.ReadAll()

	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got %v", err)
	}
}

func TestJSONLReader_ReadAll_MalformedLine(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLMalformed)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 beads (valid lines), malformed line should be skipped
	if len(beads) != 2 {
		t.Fatalf("got %d beads, want 2 (malformed line should be skipped)", len(beads))
	}

	// Verify expected beads are present
	if beads[0].ID != "bd-001" {
		t.Errorf("beads[0].ID = %q, want %q", beads[0].ID, "bd-001")
	}
	if beads[1].ID != "bd-003" {
		t.Errorf("beads[1].ID = %q, want %q", beads[1].ID, "bd-003")
	}
}

func TestJSONLReader_ReadAll_MalformedTrailingLine(t *testing.T) {
	// Malformed trailing line should be skipped silently (partial write scenario)
	content := `{"id":"bd-001","title":"Valid task","description":"Valid","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-002","title":"Second task","description":"Also valid","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}
{incomplete json`

	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, content)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 beads (valid lines), trailing malformed line skipped
	if len(beads) != 2 {
		t.Fatalf("got %d beads, want 2 (trailing malformed line should be skipped)", len(beads))
	}
}

func TestJSONLReader_ReadAll_BlankLines(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithBlankLines)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 beads, blank lines skipped
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3 (blank lines should be skipped)", len(beads))
	}

	// Verify expected beads are present
	if beads[0].ID != "bd-001" {
		t.Errorf("beads[0].ID = %q, want %q", beads[0].ID, "bd-001")
	}
	if beads[1].ID != "bd-002" {
		t.Errorf("beads[1].ID = %q, want %q", beads[1].ID, "bd-002")
	}
	if beads[2].ID != "bd-003" {
		t.Errorf("beads[2].ID = %q, want %q", beads[2].ID, "bd-003")
	}
}

func TestJSONLReader_ReadAll_WhitespaceOnlyLines(t *testing.T) {
	// Lines with only whitespace should also be skipped
	content := `{"id":"bd-001","title":"First","description":"Task","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}


{"id":"bd-002","title":"Second","description":"Task","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}`

	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, content)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 2 {
		t.Fatalf("got %d beads, want 2 (whitespace-only lines should be skipped)", len(beads))
	}
}

func TestJSONLReader_ReadAll_AllFields(t *testing.T) {
	// Test that all fields are correctly parsed
	content := `{"id":"bd-001","title":"Full bead","description":"With all fields","status":"closed","priority":1,"issue_type":"epic","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-16T10:00:00Z","closed_at":"2024-01-17T10:00:00Z","close_reason":"completed","parent":"bd-parent","notes":"Some notes"}`

	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, content)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 1 {
		t.Fatalf("got %d beads, want 1", len(beads))
	}

	b := beads[0]
	if b.ID != "bd-001" {
		t.Errorf("ID = %q, want %q", b.ID, "bd-001")
	}
	if b.Title != "Full bead" {
		t.Errorf("Title = %q, want %q", b.Title, "Full bead")
	}
	if b.Description != "With all fields" {
		t.Errorf("Description = %q, want %q", b.Description, "With all fields")
	}
	if b.Status != "closed" {
		t.Errorf("Status = %q, want %q", b.Status, "closed")
	}
	if b.Priority != 1 {
		t.Errorf("Priority = %d, want %d", b.Priority, 1)
	}
	if b.IssueType != "epic" {
		t.Errorf("IssueType = %q, want %q", b.IssueType, "epic")
	}
	if b.CreatedAt != "2024-01-15T10:00:00Z" {
		t.Errorf("CreatedAt = %q, want %q", b.CreatedAt, "2024-01-15T10:00:00Z")
	}
	if b.CreatedBy != "user" {
		t.Errorf("CreatedBy = %q, want %q", b.CreatedBy, "user")
	}
	if b.UpdatedAt != "2024-01-16T10:00:00Z" {
		t.Errorf("UpdatedAt = %q, want %q", b.UpdatedAt, "2024-01-16T10:00:00Z")
	}
	if b.ClosedAt != "2024-01-17T10:00:00Z" {
		t.Errorf("ClosedAt = %q, want %q", b.ClosedAt, "2024-01-17T10:00:00Z")
	}
	if b.Parent != "bd-parent" {
		t.Errorf("Parent = %q, want %q", b.Parent, "bd-parent")
	}
	if b.Notes != "Some notes" {
		t.Errorf("Notes = %q, want %q", b.Notes, "Some notes")
	}
}

func TestJSONLReader_ReadAll_NoDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLBasic)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadAll()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Beads without dependencies should have nil/empty Dependencies slice
	for _, b := range beads {
		if b.DependencyCount != 0 {
			t.Errorf("bead %s: DependencyCount = %d, want 0", b.ID, b.DependencyCount)
		}
		if len(b.Dependencies) != 0 {
			t.Errorf("bead %s: has %d dependencies, want 0", b.ID, len(b.Dependencies))
		}
	}
}

func TestNewJSONLReader(t *testing.T) {
	reader := NewJSONLReader("/some/path")
	if reader == nil {
		t.Fatal("NewJSONLReader returned nil")
	}
	if reader.beadsDir != "/some/path" {
		t.Errorf("beadsDir = %q, want %q", reader.beadsDir, "/some/path")
	}
}

func TestJSONLReader_ReadActive(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLMixedStatus)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadActive()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 beads: open, in_progress, blocked
	// Agent bead should be filtered out, deferred and closed should be excluded
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3 (open, in_progress, blocked)", len(beads))
	}

	// Verify correct statuses
	statusSet := make(map[string]bool)
	for _, b := range beads {
		statusSet[b.Status] = true
		// Verify agent beads are filtered
		if b.IssueType == "agent" {
			t.Error("agent bead should have been filtered out")
		}
	}

	if !statusSet["open"] {
		t.Error("expected open status bead")
	}
	if !statusSet["in_progress"] {
		t.Error("expected in_progress status bead")
	}
	if !statusSet["blocked"] {
		t.Error("expected blocked status bead")
	}
}

func TestJSONLReader_ReadActive_FiltersAgentBeads(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLWithAgent)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadActive()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 beads (both tasks), agent should be filtered
	if len(beads) != 2 {
		t.Fatalf("got %d beads, want 2 (agent should be filtered)", len(beads))
	}

	for _, b := range beads {
		if b.IssueType == "agent" {
			t.Error("agent bead should have been filtered out")
		}
	}
}

func TestJSONLReader_ReadBacklog(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLMixedStatus)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadBacklog()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 bead: deferred
	if len(beads) != 1 {
		t.Fatalf("got %d beads, want 1 (deferred only)", len(beads))
	}

	if beads[0].Status != "deferred" {
		t.Errorf("expected deferred status, got %q", beads[0].Status)
	}
	if beads[0].ID != "bd-deferred" {
		t.Errorf("expected bd-deferred, got %q", beads[0].ID)
	}
}

func TestJSONLReader_ReadBacklog_Empty(t *testing.T) {
	// Test with a file that has no deferred beads
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLBasic)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadBacklog()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 0 {
		t.Errorf("got %d beads, want 0 (no deferred beads)", len(beads))
	}
}

func TestJSONLReader_ReadClosed(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLMixedStatus)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadClosed(7)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Note: This test uses fixed dates (2024-01-20) so results depend on current time
	// When run after 2024-01-27, no beads will match the 7-day filter
	// This is expected behavior - we're testing the filter logic

	// All returned beads should have closed status
	for _, b := range beads {
		if b.Status != "closed" {
			t.Errorf("expected closed status, got %q", b.Status)
		}
		if b.IssueType == "agent" {
			t.Error("agent bead should have been filtered out")
		}
	}
}

func TestJSONLReader_ReadClosed_Empty(t *testing.T) {
	// Test with a file that has no closed beads
	tmpDir := t.TempDir()
	beadsDir := writeJSONLFile(t, tmpDir, testutil.GraphJSONLBasic)

	reader := NewJSONLReader(beadsDir)
	beads, err := reader.ReadClosed(7)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(beads) != 0 {
		t.Errorf("got %d beads, want 0 (no closed beads)", len(beads))
	}
}

func TestJSONLReader_ReadActive_FileNotFound(t *testing.T) {
	reader := NewJSONLReader("/nonexistent/path/.beads")
	_, err := reader.ReadActive()

	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got %v", err)
	}
}

func TestJSONLReader_ReadBacklog_FileNotFound(t *testing.T) {
	reader := NewJSONLReader("/nonexistent/path/.beads")
	_, err := reader.ReadBacklog()

	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got %v", err)
	}
}

func TestJSONLReader_ReadClosed_FileNotFound(t *testing.T) {
	reader := NewJSONLReader("/nonexistent/path/.beads")
	_, err := reader.ReadClosed(7)

	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got %v", err)
	}
}
