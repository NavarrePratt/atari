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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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
	mock.SetResponse("bd", []string{"ready", "--json"}, []byte(`[
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
	mock.SetResponse("bd", []string{"ready", "--json"}, []byte(`[
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
	testutil.SetupMockBDReady(mock, testutil.EmptyBeadReadyJSON)

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
	testutil.SetupMockBDReady(mock, testutil.SingleBeadReadyJSON)

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
	testutil.SetupMockBDReady(mock, testutil.SingleBeadReadyJSON)

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
	testutil.SetupMockBDReady(mock, testutil.SingleBeadReadyJSON)

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
	mock.SetResponse("bd", []string{"ready", "--json", "--unassigned"}, []byte(testutil.SingleBeadReadyJSON))

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

	testutil.AssertCalled(t, mock, "bd", "ready", "--json", "--unassigned")
}

func TestPoll_WithLabelAndUnassigned(t *testing.T) {
	mock := testutil.NewMockRunner()
	mock.SetResponse("bd", []string{"ready", "--json", "--label", "automated", "--unassigned"}, []byte(testutil.SingleBeadReadyJSON))

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

	testutil.AssertCalled(t, mock, "bd", "ready", "--json", "--label", "automated", "--unassigned")
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

	eligible := m.filterEligible(beads)
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

	eligible := m.filterEligible(beads)
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
