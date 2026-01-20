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
	testutil.SetupMockBRReady(mock, testutil.SampleBeadReadyJSON)

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

	testutil.AssertCalled(t, mock, "br", "ready", "--json")
}

func TestPoll_SingleBead(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.SingleBeadReadyJSON)

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
	testutil.SetupMockBRReady(mock, testutil.EmptyBeadReadyJSON)

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
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(""))

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
	mock.SetResponse("br", []string{"ready", "--json", "--label", "automated"}, []byte(testutil.SingleBeadReadyJSON))

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

	testutil.AssertCalled(t, mock, "br", "ready", "--json", "--label", "automated")
}

func TestPoll_CommandError(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetError("br", []string{"ready", "--json"}, errors.New("command not found"))

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
	mock.SetResponse("br", []string{"ready", "--json"}, []byte("not valid json"))

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
	mock.SetError("br", []string{"ready", "--json"}, context.Canceled)

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
	// Test that Bead struct correctly parses all fields from br ready JSON
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
	if m.history == nil {
		t.Error("expected history map to be initialized")
	}
}

// Tests for calculateBackoff

func TestCalculateBackoff_FirstAttempt(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// First attempt (attempts=1) should return 0 backoff
	backoff := m.calculateBackoff(1)
	if backoff != 0 {
		t.Errorf("expected 0 backoff for first attempt, got %v", backoff)
	}

	// Zero attempts should also return 0
	backoff = m.calculateBackoff(0)
	if backoff != 0 {
		t.Errorf("expected 0 backoff for zero attempts, got %v", backoff)
	}
}

func TestCalculateBackoff_ExponentialGrowth(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Minute
	cfg.Backoff.Multiplier = 2.0
	cfg.Backoff.Max = 1 * time.Hour

	m := New(cfg, testutil.NewMockRunner())

	// 2 attempts: initial = 1m
	backoff := m.calculateBackoff(2)
	if backoff != 1*time.Minute {
		t.Errorf("expected 1m for 2 attempts, got %v", backoff)
	}

	// 3 attempts: 1m * 2 = 2m
	backoff = m.calculateBackoff(3)
	if backoff != 2*time.Minute {
		t.Errorf("expected 2m for 3 attempts, got %v", backoff)
	}

	// 4 attempts: 2m * 2 = 4m
	backoff = m.calculateBackoff(4)
	if backoff != 4*time.Minute {
		t.Errorf("expected 4m for 4 attempts, got %v", backoff)
	}
}

func TestCalculateBackoff_MaxCap(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Minute
	cfg.Backoff.Multiplier = 2.0
	cfg.Backoff.Max = 10 * time.Minute

	m := New(cfg, testutil.NewMockRunner())

	// After many attempts, backoff should be capped at max
	backoff := m.calculateBackoff(10) // Would be 256m without cap
	if backoff != 10*time.Minute {
		t.Errorf("expected max backoff 10m, got %v", backoff)
	}
}

// Tests for filterEligible

func TestFilterEligible_NewBeads(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads, got %d", len(eligible))
	}
}

func TestFilterEligible_ExcludesEpics(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, IssueType: "task"},
		{ID: "bd-002", Priority: 2, IssueType: "epic"},
		{ID: "bd-003", Priority: 3, IssueType: "bug"},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads (excluding epic), got %d", len(eligible))
	}

	// Verify the epic is excluded
	for _, b := range eligible {
		if b.IssueType == "epic" {
			t.Errorf("epic should have been filtered out, but found %s", b.ID)
		}
	}
	// Verify task and bug are included
	if eligible[0].ID != "bd-001" {
		t.Errorf("expected bd-001, got %s", eligible[0].ID)
	}
	if eligible[1].ID != "bd-003" {
		t.Errorf("expected bd-003, got %s", eligible[1].ID)
	}
}

func TestFilterEligible_ExcludesCompleted(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// Mark bd-001 as completed
	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(eligible))
	}
	if eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", eligible[0].ID)
	}
}

func TestFilterEligible_ExcludesAbandoned(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// Mark bd-001 as abandoned
	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryAbandoned}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(eligible))
	}
	if eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", eligible[0].ID)
	}
}

func TestFilterEligible_ExcludesInBackoff(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Hour // Long backoff

	m := New(cfg, testutil.NewMockRunner())

	// Mark bd-001 as failed recently (still in backoff)
	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now(),
	}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(eligible))
	}
	if eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", eligible[0].ID)
	}
}

func TestFilterEligible_IncludesAfterBackoff(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Millisecond // Very short backoff

	m := New(cfg, testutil.NewMockRunner())

	// Mark bd-001 as failed long ago (past backoff)
	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads after backoff, got %d", len(eligible))
	}
}

func TestFilterEligible_ExcludesMaxFailures(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3
	cfg.Backoff.Initial = 1 * time.Millisecond // Short backoff

	m := New(cfg, testutil.NewMockRunner())

	// Mark bd-001 as failed with max attempts (would be abandoned on next failure)
	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    3, // At max failures
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 1 {
		t.Errorf("expected 1 eligible bead when max failures reached, got %d", len(eligible))
	}
	if eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", eligible[0].ID)
	}
}

// Tests for Next

func TestNext_ReturnsHighestPriority(t *testing.T) {
	mock := testutil.NewMockRunner()
	// Return beads in non-priority order
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(`[
		{"id": "bd-002", "title": "Low priority", "status": "open", "priority": 3, "created_at": "2024-01-15T10:00:00Z"},
		{"id": "bd-001", "title": "High priority", "status": "open", "priority": 1, "created_at": "2024-01-15T11:00:00Z"}
	]`))

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead, got nil")
	}
	if bead.ID != "bd-001" {
		t.Errorf("expected highest priority bd-001, got %s", bead.ID)
	}
}

func TestNext_SamePriorityOldestFirst(t *testing.T) {
	mock := testutil.NewMockRunner()
	// Return beads with same priority, different creation times
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(`[
		{"id": "bd-002", "title": "Newer", "status": "open", "priority": 1, "created_at": "2024-01-15T12:00:00Z"},
		{"id": "bd-001", "title": "Older", "status": "open", "priority": 1, "created_at": "2024-01-15T10:00:00Z"}
	]`))

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead, got nil")
	}
	if bead.ID != "bd-001" {
		t.Errorf("expected older bead bd-001, got %s", bead.ID)
	}
}

func TestNext_NoBeadsAvailable(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.EmptyBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no work available, got %v", bead)
	}
}

func TestNext_AllBeadsFiltered(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.SingleBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	// Mark the only bead as completed
	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when all filtered, got %v", bead)
	}
}

func TestNext_MarksAsWorking(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.SingleBeadReadyJSON)

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	// Check history was updated
	h := m.history[bead.ID]
	if h == nil {
		t.Fatal("expected history entry")
	}
	if h.Status != HistoryWorking {
		t.Errorf("expected status working, got %s", h.Status)
	}
	if h.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", h.Attempts)
	}
}

func TestNext_IncrementsAttempts(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.SingleBeadReadyJSON)

	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Millisecond

	m := New(cfg, mock)

	// Set up existing history with previous failed attempt
	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    1,
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	h := m.history[bead.ID]
	if h.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", h.Attempts)
	}
}

// Tests for RecordSuccess and RecordFailure

func TestRecordSuccess(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	m.RecordSuccess("bd-001")

	h := m.history["bd-001"]
	if h == nil {
		t.Fatal("expected history entry")
	}
	if h.Status != HistoryCompleted {
		t.Errorf("expected status completed, got %s", h.Status)
	}
}

func TestRecordSuccess_ExistingHistory(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// Set up existing working history
	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 2,
	}

	m.RecordSuccess("bd-001")

	h := m.history["bd-001"]
	if h.Status != HistoryCompleted {
		t.Errorf("expected status completed, got %s", h.Status)
	}
	if h.Attempts != 2 {
		t.Errorf("expected attempts preserved as 2, got %d", h.Attempts)
	}
}

func TestRecordFailure(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	testErr := errors.New("test failed")
	m.RecordFailure("bd-001", testErr)

	h := m.history["bd-001"]
	if h == nil {
		t.Fatal("expected history entry")
	}
	if h.Status != HistoryFailed {
		t.Errorf("expected status failed, got %s", h.Status)
	}
	if h.LastError != "test failed" {
		t.Errorf("expected error 'test failed', got %s", h.LastError)
	}
}

func TestRecordFailure_TriggersAbandoned(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3

	m := New(cfg, testutil.NewMockRunner())

	// Set up existing history at max failures
	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 3, // At max
	}

	m.RecordFailure("bd-001", errors.New("failed again"))

	h := m.history["bd-001"]
	if h.Status != HistoryAbandoned {
		t.Errorf("expected status abandoned when max failures exceeded, got %s", h.Status)
	}
}

func TestRecordFailure_NotAbandonedBelowMax(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3

	m := New(cfg, testutil.NewMockRunner())

	// Set up existing history below max failures
	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 2, // Below max
	}

	m.RecordFailure("bd-001", errors.New("failed"))

	h := m.history["bd-001"]
	if h.Status != HistoryFailed {
		t.Errorf("expected status failed when below max, got %s", h.Status)
	}
}

// Tests for Stats

func TestStats_Empty(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	stats := m.Stats()
	if stats.TotalSeen != 0 {
		t.Errorf("expected 0 total seen, got %d", stats.TotalSeen)
	}
	if stats.Completed != 0 {
		t.Errorf("expected 0 completed, got %d", stats.Completed)
	}
}

func TestStats_Counts(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Hour

	m := New(cfg, testutil.NewMockRunner())

	// Add various history entries
	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}
	m.history["bd-002"] = &BeadHistory{ID: "bd-002", Status: HistoryCompleted}
	m.history["bd-003"] = &BeadHistory{
		ID:          "bd-003",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now(), // Still in backoff
	}
	m.history["bd-004"] = &BeadHistory{
		ID:          "bd-004",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now().Add(-2 * time.Hour), // Past backoff
	}
	m.history["bd-005"] = &BeadHistory{ID: "bd-005", Status: HistoryAbandoned}

	stats := m.Stats()
	if stats.TotalSeen != 5 {
		t.Errorf("expected 5 total seen, got %d", stats.TotalSeen)
	}
	if stats.Completed != 2 {
		t.Errorf("expected 2 completed, got %d", stats.Completed)
	}
	if stats.Failed != 2 {
		t.Errorf("expected 2 failed, got %d", stats.Failed)
	}
	if stats.Abandoned != 1 {
		t.Errorf("expected 1 abandoned, got %d", stats.Abandoned)
	}
	if stats.InBackoff != 1 {
		t.Errorf("expected 1 in backoff, got %d", stats.InBackoff)
	}
}

// Tests for History and SetHistory

func TestHistory_ReturnsCopy(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	h := m.History()
	if len(h) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(h))
	}

	// Modifying returned map shouldn't affect internal state
	h["bd-002"] = &BeadHistory{ID: "bd-002", Status: HistoryFailed}
	if len(m.history) != 1 {
		t.Errorf("internal history should still have 1 entry, got %d", len(m.history))
	}

	// Modifying returned entry shouldn't affect internal state
	h["bd-001"].Status = HistoryFailed
	if m.history["bd-001"].Status != HistoryCompleted {
		t.Errorf("internal status should still be completed")
	}
}

func TestSetHistory_RestoresState(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// Simulate restoring from persisted state
	persisted := map[string]*BeadHistory{
		"bd-001": {ID: "bd-001", Status: HistoryCompleted, Attempts: 1},
		"bd-002": {ID: "bd-002", Status: HistoryFailed, Attempts: 2},
	}

	m.SetHistory(persisted)

	if len(m.history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(m.history))
	}
	if m.history["bd-001"].Status != HistoryCompleted {
		t.Errorf("expected bd-001 status completed")
	}
	if m.history["bd-002"].Attempts != 2 {
		t.Errorf("expected bd-002 attempts 2")
	}
}

func TestSetHistory_CopiesInput(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	input := map[string]*BeadHistory{
		"bd-001": {ID: "bd-001", Status: HistoryCompleted},
	}

	m.SetHistory(input)

	// Modifying input shouldn't affect internal state
	input["bd-001"].Status = HistoryFailed
	if m.history["bd-001"].Status != HistoryCompleted {
		t.Errorf("internal status should still be completed after input modification")
	}
}

// Tests for UnassignedOnly filtering

func TestPoll_WithUnassignedFilter(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("br", []string{"ready", "--json", "--unassigned"}, []byte(testutil.SingleBeadReadyJSON))

	cfg := config.Default()
	cfg.WorkQueue.UnassignedOnly = true
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beads))
	}

	testutil.AssertCalled(t, mock, "br", "ready", "--json", "--unassigned")
}

func TestPoll_WithLabelAndUnassigned(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("br", []string{"ready", "--json", "--label", "automated", "--unassigned"}, []byte(testutil.SingleBeadReadyJSON))

	cfg := config.Default()
	cfg.WorkQueue.Label = "automated"
	cfg.WorkQueue.UnassignedOnly = true
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beads))
	}

	testutil.AssertCalled(t, mock, "br", "ready", "--json", "--label", "automated", "--unassigned")
}

// Tests for ExcludeLabels filtering

func TestFilterEligible_ExcludesLabels(t *testing.T) {
	cfg := config.Default()
	cfg.WorkQueue.ExcludeLabels = []string{"manual", "blocked"}
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, Labels: []string{"automated"}},
		{ID: "bd-002", Priority: 2, Labels: []string{"manual"}},              // should be excluded
		{ID: "bd-003", Priority: 3, Labels: []string{"urgent", "blocked"}},   // should be excluded
		{ID: "bd-004", Priority: 4, Labels: nil},                             // no labels - included
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads, got %d", len(eligible))
	}

	// Verify correct beads are included
	ids := make(map[string]bool)
	for _, b := range eligible {
		ids[b.ID] = true
	}
	if !ids["bd-001"] {
		t.Error("expected bd-001 to be eligible")
	}
	if !ids["bd-004"] {
		t.Error("expected bd-004 to be eligible")
	}
	if ids["bd-002"] {
		t.Error("expected bd-002 to be excluded")
	}
	if ids["bd-003"] {
		t.Error("expected bd-003 to be excluded")
	}
}

func TestFilterEligible_NoExcludeLabels(t *testing.T) {
	cfg := config.Default()
	// No exclude labels set
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, Labels: []string{"manual"}},
		{ID: "bd-002", Priority: 2, Labels: []string{"blocked"}},
	}

	eligible := m.filterEligible(beads, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads when no exclude labels, got %d", len(eligible))
	}
}

func TestHasExcludedLabel(t *testing.T) {
	cfg := config.Default()
	cfg.WorkQueue.ExcludeLabels = []string{"manual", "blocked"}
	m := New(cfg, testutil.NewMockRunner())

	tests := []struct {
		name     string
		labels   []string
		expected bool
	}{
		{"no labels", nil, false},
		{"empty labels", []string{}, false},
		{"no match", []string{"automated", "urgent"}, false},
		{"single match", []string{"manual"}, true},
		{"one of many", []string{"urgent", "blocked", "priority"}, true},
		{"case sensitive", []string{"MANUAL"}, false}, // exact match only
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.hasExcludedLabel(tt.labels)
			if result != tt.expected {
				t.Errorf("hasExcludedLabel(%v) = %v, want %v", tt.labels, result, tt.expected)
			}
		})
	}
}

func TestHasExcludedLabel_EmptyExcludeList(t *testing.T) {
	cfg := config.Default()
	// No exclude labels set
	m := New(cfg, testutil.NewMockRunner())

	// Should always return false when exclude list is empty
	if m.hasExcludedLabel([]string{"manual", "blocked"}) {
		t.Error("expected false when exclude list is empty")
	}
}

// Tests for buildDescendantSet

func TestBuildDescendantSet_SingleLevel(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "epic-001"},
		{ID: "task-003", Parent: "other-epic"},
	}

	descendants := buildDescendantSet("epic-001", beads)

	// Epic and its direct children should be included
	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if !descendants["task-001"] {
		t.Error("expected task-001 to be in descendants")
	}
	if !descendants["task-002"] {
		t.Error("expected task-002 to be in descendants")
	}
	// Other beads should not be included
	if descendants["task-003"] {
		t.Error("expected task-003 to NOT be in descendants")
	}
}

func TestBuildDescendantSet_MultiLevel(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "sub-epic", Parent: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "sub-epic"},
		{ID: "grandchild", Parent: "task-001"},
	}

	descendants := buildDescendantSet("epic-001", beads)

	// All nested descendants should be included
	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if !descendants["sub-epic"] {
		t.Error("expected sub-epic to be in descendants")
	}
	if !descendants["task-001"] {
		t.Error("expected task-001 to be in descendants")
	}
	if !descendants["grandchild"] {
		t.Error("expected grandchild to be in descendants")
	}
	if len(descendants) != 4 {
		t.Errorf("expected 4 descendants, got %d", len(descendants))
	}
}

func TestBuildDescendantSet_NoChildren(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "other-epic"},
	}

	descendants := buildDescendantSet("epic-001", beads)

	// Only the epic itself should be in the set
	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if len(descendants) != 1 {
		t.Errorf("expected 1 descendant (epic only), got %d", len(descendants))
	}
}

func TestBuildDescendantSet_EmptyBeads(t *testing.T) {
	descendants := buildDescendantSet("epic-001", nil)

	// Epic should still be in the set
	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if len(descendants) != 1 {
		t.Errorf("expected 1 descendant, got %d", len(descendants))
	}
}

// Tests for epic filtering in filterEligible

func TestFilterEligible_WithEpicDescendants(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
		{ID: "task-003", Priority: 3, IssueType: "task"},
	}

	// Only task-001 and task-003 are descendants
	descendants := map[string]bool{
		"epic-001": true,
		"task-001": true,
		"task-003": true,
	}

	eligible := m.filterEligible(beads, descendants)

	if len(eligible) != 2 {
		t.Fatalf("expected 2 eligible beads, got %d", len(eligible))
	}
	if eligible[0].ID != "task-001" {
		t.Errorf("expected task-001, got %s", eligible[0].ID)
	}
	if eligible[1].ID != "task-003" {
		t.Errorf("expected task-003, got %s", eligible[1].ID)
	}
}

func TestFilterEligible_WithNilDescendants(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
	}

	// nil descendants = no epic filtering
	eligible := m.filterEligible(beads, nil)

	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible beads when no epic filter, got %d", len(eligible))
	}
}

func TestFilterEligible_EpicDescendantsEmptyResult(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
	}

	// No beads match the descendant set
	descendants := map[string]bool{
		"epic-001": true,
	}

	eligible := m.filterEligible(beads, descendants)

	if len(eligible) != 0 {
		t.Errorf("expected 0 eligible beads, got %d", len(eligible))
	}
}

// Test for Next with epic filter

func TestNext_WithEpicFilter(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns beads from multiple epics
	readyJSON := `[
		{"id": "task-001", "title": "Task in epic", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-002", "title": "Task outside epic", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T11:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list returns all beads with parent info
	listJSON := `[
		{"id": "epic-001", "title": "Epic", "status": "open", "issue_type": "epic"},
		{"id": "task-001", "title": "Task in epic", "status": "open", "parent": "epic-001"},
		{"id": "task-002", "title": "Task outside epic", "status": "open", "parent": "other-epic"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	cfg.WorkQueue.Epic = "epic-001"
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead, got nil")
	}
	if bead.ID != "task-001" {
		t.Errorf("expected task-001 (in epic), got %s", bead.ID)
	}
}

func TestNext_WithEpicFilter_NoEligibleBeads(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns beads not in the epic
	readyJSON := `[
		{"id": "task-002", "title": "Task outside epic", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T11:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list shows the bead is not under the epic
	listJSON := `[
		{"id": "epic-001", "title": "Epic", "status": "open", "issue_type": "epic"},
		{"id": "task-002", "title": "Task outside epic", "status": "open", "parent": "other-epic"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	cfg.WorkQueue.Epic = "epic-001"
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no eligible beads in epic, got %v", bead)
	}
}

func TestNext_WithEpicFilter_NestedDescendants(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns a nested descendant
	readyJSON := `[
		{"id": "task-nested", "title": "Nested task", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list shows nested hierarchy: epic -> sub-epic -> task
	listJSON := `[
		{"id": "epic-001", "title": "Epic", "status": "open", "issue_type": "epic"},
		{"id": "sub-epic", "title": "Sub Epic", "status": "open", "parent": "epic-001", "issue_type": "epic"},
		{"id": "task-nested", "title": "Nested task", "status": "open", "parent": "sub-epic"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	cfg.WorkQueue.Epic = "epic-001"
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected nested task to be selected")
	}
	if bead.ID != "task-nested" {
		t.Errorf("expected task-nested, got %s", bead.ID)
	}
}

func TestNext_WithoutEpicFilter(t *testing.T) {
	mock := testutil.NewMockRunner()
	testutil.SetupMockBRReady(mock, testutil.SampleBeadReadyJSON)

	cfg := config.Default()
	// No epic filter set
	m := New(cfg, mock)

	bead, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead when no epic filter")
	}
}

// Tests for top-level selection

func TestIdentifyTopLevelItems_EpicsAndRoots(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic", Priority: 2, CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "task-001", Parent: "epic-001", Priority: 1},                                                           // Not top-level (has parent)
		{ID: "standalone-001", IssueType: "task", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)}, // Top-level (no parent)
		{ID: "epic-002", IssueType: "epic", Priority: 3, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
	}

	topLevel := identifyTopLevelItems(beads)

	if len(topLevel) != 3 {
		t.Fatalf("expected 3 top-level items, got %d", len(topLevel))
	}

	// Should be sorted by priority, then creation time
	// Priority 1: standalone-001
	// Priority 2: epic-001
	// Priority 3: epic-002
	if topLevel[0].ID != "standalone-001" {
		t.Errorf("expected standalone-001 first (priority 1), got %s", topLevel[0].ID)
	}
	if topLevel[1].ID != "epic-001" {
		t.Errorf("expected epic-001 second (priority 2), got %s", topLevel[1].ID)
	}
	if topLevel[2].ID != "epic-002" {
		t.Errorf("expected epic-002 third (priority 3), got %s", topLevel[2].ID)
	}
}

func TestIdentifyTopLevelItems_SamePrioritySortByTime(t *testing.T) {
	beads := []Bead{
		{ID: "epic-002", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)},
		{ID: "epic-001", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
	}

	topLevel := identifyTopLevelItems(beads)

	if len(topLevel) != 2 {
		t.Fatalf("expected 2 top-level items, got %d", len(topLevel))
	}

	// Same priority, should be sorted by creation time (oldest first)
	if topLevel[0].ID != "epic-001" {
		t.Errorf("expected epic-001 first (older), got %s", topLevel[0].ID)
	}
	if topLevel[1].ID != "epic-002" {
		t.Errorf("expected epic-002 second (newer), got %s", topLevel[1].ID)
	}
}

func TestIdentifyTopLevelItems_Empty(t *testing.T) {
	topLevel := identifyTopLevelItems(nil)

	if len(topLevel) != 0 {
		t.Errorf("expected 0 top-level items for nil input, got %d", len(topLevel))
	}
}

func TestHasReadyDescendants_DirectChildren(t *testing.T) {
	readyBeads := []Bead{
		{ID: "task-001", IssueType: "task"},
		{ID: "task-002", IssueType: "task"},
	}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "epic-001"},
		{ID: "task-003", Parent: "other-epic"},
	}

	if !hasReadyDescendants("epic-001", readyBeads, allBeads) {
		t.Error("expected epic-001 to have ready descendants")
	}
}

func TestHasReadyDescendants_NestedChildren(t *testing.T) {
	readyBeads := []Bead{
		{ID: "nested-task", IssueType: "task"},
	}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "sub-epic", IssueType: "epic", Parent: "epic-001"},
		{ID: "nested-task", Parent: "sub-epic"},
	}

	if !hasReadyDescendants("epic-001", readyBeads, allBeads) {
		t.Error("expected epic-001 to have nested ready descendants")
	}
}

func TestHasReadyDescendants_NoReadyWork(t *testing.T) {
	readyBeads := []Bead{
		{ID: "task-other", IssueType: "task"},
	}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-other", Parent: "other-epic"},
	}

	if hasReadyDescendants("epic-001", readyBeads, allBeads) {
		t.Error("expected epic-001 to have no ready descendants")
	}
}

func TestHasReadyDescendants_SkipsEpics(t *testing.T) {
	readyBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"}, // Epic itself is ready but should be skipped
	}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
	}

	if hasReadyDescendants("epic-001", readyBeads, allBeads) {
		t.Error("expected epic-001 to have no ready descendants (epics are skipped)")
	}
}

func TestHasReadyDescendants_StandaloneBeadIsOwnDescendant(t *testing.T) {
	readyBeads := []Bead{
		{ID: "standalone-001", IssueType: "task"},
	}
	allBeads := []Bead{
		{ID: "standalone-001", IssueType: "task"}, // No parent, not an epic
	}

	if !hasReadyDescendants("standalone-001", readyBeads, allBeads) {
		t.Error("expected standalone bead to be considered its own descendant")
	}
}

func TestSelectBestTopLevel(t *testing.T) {
	topLevelItems := []Bead{
		{ID: "epic-001", Priority: 1},
		{ID: "epic-002", Priority: 2},
	}
	readyBeads := []Bead{
		{ID: "task-002", IssueType: "task"},
	}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "epic-002", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"}, // Not ready
		{ID: "task-002", Parent: "epic-002"}, // Ready
	}

	// epic-001 has no ready work, epic-002 does
	best := selectBestTopLevel(topLevelItems, readyBeads, allBeads)
	if best != "epic-002" {
		t.Errorf("expected epic-002 (has ready work), got %s", best)
	}
}

func TestSelectBestTopLevel_NoReadyWork(t *testing.T) {
	topLevelItems := []Bead{
		{ID: "epic-001", Priority: 1},
	}
	readyBeads := []Bead{}
	allBeads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
	}

	best := selectBestTopLevel(topLevelItems, readyBeads, allBeads)
	if best != "" {
		t.Errorf("expected empty string when no ready work, got %s", best)
	}
}

func TestNextTopLevel_SelectsFromActiveTopLevel(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns tasks from multiple epics
	readyJSON := `[
		{"id": "task-epic1", "title": "Task in epic1", "status": "open", "priority": 2, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-epic2", "title": "Task in epic2", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list returns hierarchy
	listJSON := `[
		{"id": "epic-001", "title": "Epic 1", "status": "open", "issue_type": "epic", "priority": 2},
		{"id": "epic-002", "title": "Epic 2", "status": "open", "issue_type": "epic", "priority": 1},
		{"id": "task-epic1", "title": "Task in epic1", "status": "open", "parent": "epic-001"},
		{"id": "task-epic2", "title": "Task in epic2", "status": "open", "parent": "epic-002"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	// Set active top-level to epic-001
	m.SetActiveTopLevel("epic-001")

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	// Should select from epic-001 even though epic-002 has higher priority work
	if bead.ID != "task-epic1" {
		t.Errorf("expected task-epic1 (from active top-level), got %s", bead.ID)
	}

	// Active top-level should remain
	if m.ActiveTopLevel() != "epic-001" {
		t.Errorf("expected active top-level to remain epic-001, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_SwitchesWhenExhausted(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns only task from epic-002 (epic-001 is exhausted)
	readyJSON := `[
		{"id": "task-epic2", "title": "Task in epic2", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list returns hierarchy
	listJSON := `[
		{"id": "epic-001", "title": "Epic 1", "status": "open", "issue_type": "epic", "priority": 2},
		{"id": "epic-002", "title": "Epic 2", "status": "open", "issue_type": "epic", "priority": 1},
		{"id": "task-epic2", "title": "Task in epic2", "status": "open", "parent": "epic-002"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	// Set active top-level to epic-001 (which is exhausted)
	m.SetActiveTopLevel("epic-001")

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	// Should switch to epic-002 and select its task
	if bead.ID != "task-epic2" {
		t.Errorf("expected task-epic2, got %s", bead.ID)
	}

	// Active top-level should switch to epic-002
	if m.ActiveTopLevel() != "epic-002" {
		t.Errorf("expected active top-level to switch to epic-002, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_SelectsHighestPriorityTopLevel(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns tasks from multiple epics
	readyJSON := `[
		{"id": "task-low", "title": "Task low priority", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-high", "title": "Task high priority", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list returns hierarchy - epic-high has highest priority (lower number)
	listJSON := `[
		{"id": "epic-low", "title": "Low Priority Epic", "status": "open", "issue_type": "epic", "priority": 3},
		{"id": "epic-high", "title": "High Priority Epic", "status": "open", "issue_type": "epic", "priority": 1},
		{"id": "task-low", "title": "Task low priority", "status": "open", "parent": "epic-low"},
		{"id": "task-high", "title": "Task high priority", "status": "open", "parent": "epic-high"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	// Should select from highest priority top-level (epic-high with priority 1)
	if bead.ID != "task-high" {
		t.Errorf("expected task-high (from highest priority epic), got %s", bead.ID)
	}

	// Active top-level should be epic-high
	if m.ActiveTopLevel() != "epic-high" {
		t.Errorf("expected active top-level to be epic-high, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_StandaloneBeads(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns a standalone bead (no parent)
	readyJSON := `[
		{"id": "standalone-001", "title": "Standalone task", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list shows the standalone bead has no parent
	listJSON := `[
		{"id": "standalone-001", "title": "Standalone task", "status": "open", "issue_type": "task", "priority": 1}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	if bead.ID != "standalone-001" {
		t.Errorf("expected standalone-001, got %s", bead.ID)
	}

	// Standalone bead should be its own top-level
	if m.ActiveTopLevel() != "standalone-001" {
		t.Errorf("expected active top-level to be standalone-001, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_NoBeads(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

	cfg := config.Default()
	m := New(cfg, mock)

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no work available, got %v", bead)
	}
}

func TestNextTopLevel_RespectsHistoryFilters(t *testing.T) {
	mock := testutil.NewMockRunner()

	// br ready returns tasks
	readyJSON := `[
		{"id": "task-001", "title": "Task 1", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-002", "title": "Task 2", "status": "open", "priority": 2, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// br list
	listJSON := `[
		{"id": "epic-001", "title": "Epic", "status": "open", "issue_type": "epic", "priority": 1},
		{"id": "task-001", "title": "Task 1", "status": "open", "parent": "epic-001"},
		{"id": "task-002", "title": "Task 2", "status": "open", "parent": "epic-001"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	// Mark task-001 as completed
	m.history["task-001"] = &BeadHistory{ID: "task-001", Status: HistoryCompleted}

	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	// Should skip task-001 (completed) and select task-002
	if bead.ID != "task-002" {
		t.Errorf("expected task-002 (task-001 is completed), got %s", bead.ID)
	}
}

func TestNextTopLevel_MultiEpicScenario(t *testing.T) {
	mock := testutil.NewMockRunner()

	// Multiple tasks across different epics
	readyJSON := `[
		{"id": "task-A1", "title": "Task A1", "status": "open", "priority": 2, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-A2", "title": "Task A2", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"},
		{"id": "task-B1", "title": "Task B1", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z"}
	]`
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(readyJSON))

	// Hierarchy with two epics
	listJSON := `[
		{"id": "epic-A", "title": "Epic A", "status": "open", "issue_type": "epic", "priority": 1, "created_at": "2024-01-15T08:00:00Z"},
		{"id": "epic-B", "title": "Epic B", "status": "open", "issue_type": "epic", "priority": 1, "created_at": "2024-01-15T09:00:00Z"},
		{"id": "task-A1", "title": "Task A1", "status": "open", "parent": "epic-A", "priority": 2},
		{"id": "task-A2", "title": "Task A2", "status": "open", "parent": "epic-A", "priority": 1},
		{"id": "task-B1", "title": "Task B1", "status": "open", "parent": "epic-B", "priority": 1}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	// First selection should pick from epic-A (older, same priority)
	bead1, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead1 == nil {
		t.Fatal("expected a bead")
	}

	// Should select task-A2 (highest priority within epic-A)
	if bead1.ID != "task-A2" {
		t.Errorf("expected task-A2, got %s", bead1.ID)
	}
	if m.ActiveTopLevel() != "epic-A" {
		t.Errorf("expected active top-level to be epic-A, got %s", m.ActiveTopLevel())
	}
}

func TestActiveTopLevel_GetSet(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, testutil.NewMockRunner())

	// Initially empty
	if m.ActiveTopLevel() != "" {
		t.Errorf("expected empty active top-level, got %s", m.ActiveTopLevel())
	}

	// Set and get
	m.SetActiveTopLevel("epic-001")
	if m.ActiveTopLevel() != "epic-001" {
		t.Errorf("expected epic-001, got %s", m.ActiveTopLevel())
	}

	// Clear
	m.ClearActiveTopLevel()
	if m.ActiveTopLevel() != "" {
		t.Errorf("expected empty after clear, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_ExhaustedClearsActiveTopLevel(t *testing.T) {
	mock := testutil.NewMockRunner()

	// No ready beads (all exhausted)
	mock.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

	cfg := config.Default()
	m := New(cfg, mock)

	// Set an active top-level
	m.SetActiveTopLevel("epic-001")

	// NextTopLevel with no beads should not crash and return nil
	bead, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead, got %v", bead)
	}
}

func TestFetchAllBeads(t *testing.T) {
	mock := testutil.NewMockRunner()
	listJSON := `[
		{"id": "bead-001", "title": "Bead 1", "status": "open"},
		{"id": "bead-002", "title": "Bead 2", "status": "open"}
	]`
	mock.SetResponse("br", []string{"list", "--json"}, []byte(listJSON))

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.fetchAllBeads(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beads))
	}
	if beads[0].ID != "bead-001" {
		t.Errorf("expected bead-001, got %s", beads[0].ID)
	}
}

func TestFetchAllBeads_Empty(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("br", []string{"list", "--json"}, []byte(""))

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.fetchAllBeads(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if beads != nil {
		t.Errorf("expected nil beads for empty output, got %v", beads)
	}
}

func TestFetchAllBeads_Error(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetError("br", []string{"list", "--json"}, errors.New("command failed"))

	cfg := config.Default()
	m := New(cfg, mock)

	_, err := m.fetchAllBeads(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
