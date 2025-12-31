package testutil

import (
	"encoding/json"
	"testing"
)

func TestSampleBeadReadyJSON_IsValidJSON(t *testing.T) {
	var beads []Bead
	if err := json.Unmarshal([]byte(SampleBeadReadyJSON), &beads); err != nil {
		t.Fatalf("SampleBeadReadyJSON is not valid JSON: %v", err)
	}
	if len(beads) != 2 {
		t.Errorf("expected 2 beads, got %d", len(beads))
	}
	if beads[0].ID != "bd-001" {
		t.Errorf("first bead ID should be 'bd-001', got %s", beads[0].ID)
	}
}

func TestSingleBeadReadyJSON_IsValidJSON(t *testing.T) {
	var beads []Bead
	if err := json.Unmarshal([]byte(SingleBeadReadyJSON), &beads); err != nil {
		t.Fatalf("SingleBeadReadyJSON is not valid JSON: %v", err)
	}
	if len(beads) != 1 {
		t.Errorf("expected 1 bead, got %d", len(beads))
	}
}

func TestEmptyBeadReadyJSON_IsValidJSON(t *testing.T) {
	var beads []Bead
	if err := json.Unmarshal([]byte(EmptyBeadReadyJSON), &beads); err != nil {
		t.Fatalf("EmptyBeadReadyJSON is not valid JSON: %v", err)
	}
	if len(beads) != 0 {
		t.Errorf("expected 0 beads, got %d", len(beads))
	}
}

func TestBDAgentStateSuccess_IsValidJSON(t *testing.T) {
	var result map[string]any
	if err := json.Unmarshal([]byte(BDAgentStateSuccess), &result); err != nil {
		t.Fatalf("BDAgentStateSuccess is not valid JSON: %v", err)
	}
}

func TestBDCloseSuccess_IsValidJSON(t *testing.T) {
	var result map[string]any
	if err := json.Unmarshal([]byte(BDCloseSuccess), &result); err != nil {
		t.Fatalf("BDCloseSuccess is not valid JSON: %v", err)
	}
}

func TestSampleClaudeEvents_AreValidJSON(t *testing.T) {
	events := []struct {
		name string
		json string
	}{
		{"SampleClaudeInit", SampleClaudeInit},
		{"SampleClaudeAssistant", SampleClaudeAssistant},
		{"SampleClaudeToolUse", SampleClaudeToolUse},
		{"SampleClaudeToolResult", SampleClaudeToolResult},
		{"SampleClaudeResultSuccess", SampleClaudeResultSuccess},
		{"SampleClaudeResultError", SampleClaudeResultError},
		{"SampleClaudeResultMaxTurns", SampleClaudeResultMaxTurns},
	}

	for _, event := range events {
		var result map[string]any
		if err := json.Unmarshal([]byte(event.json), &result); err != nil {
			t.Errorf("%s is not valid JSON: %v", event.name, err)
		}
	}
}

func TestSampleStateJSON_IsValidJSON(t *testing.T) {
	var state map[string]any
	if err := json.Unmarshal([]byte(SampleStateJSON), &state); err != nil {
		t.Fatalf("SampleStateJSON is not valid JSON: %v", err)
	}
	if state["running"] != true {
		t.Error("running should be true")
	}
	if state["current_bead"] != "bd-001" {
		t.Error("current_bead should be 'bd-001'")
	}
}

func TestEmptyStateJSON_IsValidJSON(t *testing.T) {
	var state map[string]any
	if err := json.Unmarshal([]byte(EmptyStateJSON), &state); err != nil {
		t.Fatalf("EmptyStateJSON is not valid JSON: %v", err)
	}
	if state["running"] != false {
		t.Error("running should be false")
	}
}

func TestSampleBead1_HasExpectedValues(t *testing.T) {
	if SampleBead1.ID != "bd-001" {
		t.Errorf("SampleBead1.ID = %s, want bd-001", SampleBead1.ID)
	}
	if SampleBead1.Priority != 1 {
		t.Errorf("SampleBead1.Priority = %d, want 1", SampleBead1.Priority)
	}
}

func TestSampleBead2_HasExpectedValues(t *testing.T) {
	if SampleBead2.ID != "bd-002" {
		t.Errorf("SampleBead2.ID = %s, want bd-002", SampleBead2.ID)
	}
	if SampleBead2.Priority != 2 {
		t.Errorf("SampleBead2.Priority = %d, want 2", SampleBead2.Priority)
	}
}

func TestBead_JSONRoundTrip(t *testing.T) {
	original := SampleBead1

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Bead
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, original.ID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %s, want %s", decoded.Title, original.Title)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority = %d, want %d", decoded.Priority, original.Priority)
	}
}
