package workqueue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/testutil"
)

func TestPoll_ReturnsBeads(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBDReady(mock, testutil.SampleBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beads))
	}

	// Verify first bead
	if beads[0].ID != "bd-001" {
		t.Errorf("expected ID bd-001, got %s", beads[0].ID)
	}
	if beads[0].Title != "Test bead 1" {
		t.Errorf("expected title 'Test bead 1', got %s", beads[0].Title)
	}
	if beads[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", beads[0].Priority)
	}

	// Verify second bead
	if beads[1].ID != "bd-002" {
		t.Errorf("expected ID bd-002, got %s", beads[1].ID)
	}
	if beads[1].Priority != 2 {
		t.Errorf("expected priority 2, got %d", beads[1].Priority)
	}

	testutil.AssertCalled(t, mock, "bd", "ready", "--json")
}

func TestPoll_SingleBead(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBDReady(mock, testutil.SingleBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beads))
	}
	if beads[0].ID != "bd-001" {
		t.Errorf("expected ID bd-001, got %s", beads[0].ID)
	}
}

func TestPoll_EmptyArray(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBDReady(mock, testutil.EmptyBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if beads != nil {
		t.Errorf("expected nil slice for empty array, got %v", beads)
	}
}

func TestPoll_EmptyOutput(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("bd", []string{"ready", "--json"}, []byte(""))

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if beads != nil {
		t.Errorf("expected nil slice for empty output, got %v", beads)
	}
}

func TestPoll_WithLabelFilter(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("bd", []string{"ready", "--json", "--label", "automated"}, []byte(testutil.SingleBeadReadyJSON))

	cfg := config.Default()
	cfg.WorkQueue.Label = "automated"
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beads))
	}

	testutil.AssertCalled(t, mock, "bd", "ready", "--json", "--label", "automated")
}

func TestPoll_CommandError(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetError("bd", []string{"ready", "--json"}, errors.New("command not found"))

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if beads != nil {
		t.Errorf("expected nil beads on error, got %v", beads)
	}
}

func TestPoll_InvalidJSON(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("bd", []string{"ready", "--json"}, []byte("not valid json"))

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if beads != nil {
		t.Errorf("expected nil beads on error, got %v", beads)
	}
}

func TestPoll_CanceledContext(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetError("bd", []string{"ready", "--json"}, context.Canceled)

	cfg := config.Default()
	m := New(cfg, mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	beads, err := m.Poll(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	if beads != nil {
		t.Errorf("expected nil beads on error, got %v", beads)
	}
}

func TestBead_JSONParsing(t *testing.T) {
	// Test that Bead struct correctly parses all fields from bd ready JSON
	jsonData := `{
		"id": "bd-042",
		"title": "Fix auth bug",
		"description": "Users getting logged out",
		"status": "open",
		"priority": 1,
		"issue_type": "bug",
		"labels": ["bug", "auth"],
		"created_at": "2024-01-15T10:00:00Z",
		"created_by": "user",
		"updated_at": "2024-01-15T11:00:00Z"
	}`

	var bead Bead
	if err := json.Unmarshal([]byte(jsonData), &bead); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if bead.ID != "bd-042" {
		t.Errorf("expected ID bd-042, got %s", bead.ID)
	}
	if bead.Title != "Fix auth bug" {
		t.Errorf("expected title 'Fix auth bug', got %s", bead.Title)
	}
	if bead.Description != "Users getting logged out" {
		t.Errorf("expected description 'Users getting logged out', got %s", bead.Description)
	}
	if bead.Status != "open" {
		t.Errorf("expected status 'open', got %s", bead.Status)
	}
	if bead.Priority != 1 {
		t.Errorf("expected priority 1, got %d", bead.Priority)
	}
	if bead.IssueType != "bug" {
		t.Errorf("expected issue_type 'bug', got %s", bead.IssueType)
	}
	if len(bead.Labels) != 2 || bead.Labels[0] != "bug" || bead.Labels[1] != "auth" {
		t.Errorf("expected labels [bug, auth], got %v", bead.Labels)
	}
	if bead.CreatedBy != "user" {
		t.Errorf("expected created_by 'user', got %s", bead.CreatedBy)
	}

	// Verify timestamps
	expectedCreated := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	if !bead.CreatedAt.Equal(expectedCreated) {
		t.Errorf("expected created_at %v, got %v", expectedCreated, bead.CreatedAt)
	}
	expectedUpdated := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	if !bead.UpdatedAt.Equal(expectedUpdated) {
		t.Errorf("expected updated_at %v, got %v", expectedUpdated, bead.UpdatedAt)
	}
}

func TestBead_OptionalLabels(t *testing.T) {
	// Test that Labels field is optional (omitempty)
	jsonData := `{
		"id": "bd-001",
		"title": "No labels",
		"description": "",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"created_at": "2024-01-15T10:00:00Z",
		"created_by": "user",
		"updated_at": "2024-01-15T10:00:00Z"
	}`

	var bead Bead
	if err := json.Unmarshal([]byte(jsonData), &bead); err != nil {
		t.Fatalf("failed to parse JSON without labels: %v", err)
	}

	if len(bead.Labels) != 0 {
		t.Errorf("expected empty labels, got %v", bead.Labels)
	}
}

func TestNew_CreateManager(t *testing.T) {
	mock := testutil.NewMockRunner()
	cfg := config.Default()

	m := New(cfg, mock)
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if m.config != cfg {
		t.Error("expected config to be set")
	}
	if m.runner != mock {
		t.Error("expected runner to be set")
	}
}
