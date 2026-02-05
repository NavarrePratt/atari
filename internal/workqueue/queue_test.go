package workqueue

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/npratt/atari/internal/brclient"
	"github.com/npratt/atari/internal/config"
)

func newMockClient() *brclient.MockClient {
	return brclient.NewMockClient()
}

func TestSelectionReason_String(t *testing.T) {
	tests := []struct {
		reason   SelectionReason
		expected string
	}{
		{ReasonSuccess, "success"},
		{ReasonNoReady, "no ready beads"},
		{ReasonBackoff, "all beads in backoff"},
		{ReasonMaxFailure, "all beads hit max failures"},
		{SelectionReason(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.reason.String(); got != tt.expected {
			t.Errorf("SelectionReason(%d).String() = %q, want %q", tt.reason, got, tt.expected)
		}
	}
}

func TestPoll_ReturnsBeads(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
		{ID: "bd-002", Title: "Test bead 2", Status: "open", Priority: 2},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	beads, err := m.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(beads))
	}

	if beads[0].ID != "bd-001" {
		t.Errorf("expected ID bd-001, got %s", beads[0].ID)
	}
	if beads[0].Title != "Test bead 1" {
		t.Errorf("expected title 'Test bead 1', got %s", beads[0].Title)
	}
	if beads[0].Priority != 1 {
		t.Errorf("expected priority 1, got %d", beads[0].Priority)
	}

	if beads[1].ID != "bd-002" {
		t.Errorf("expected ID bd-002, got %s", beads[1].ID)
	}
	if beads[1].Priority != 2 {
		t.Errorf("expected priority 2, got %d", beads[1].Priority)
	}
}

func TestPoll_SingleBead(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

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
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{}

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
	mock := newMockClient()
	mock.ReadyResponse = nil

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
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

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

	// Verify the options were passed
	if len(mock.ReadyCalls) != 1 {
		t.Fatalf("expected 1 Ready call, got %d", len(mock.ReadyCalls))
	}
	opts := mock.ReadyCalls[0]
	if opts == nil || opts.Label != "automated" {
		t.Errorf("expected label 'automated' in options")
	}
}

func TestPoll_CommandError(t *testing.T) {
	mock := newMockClient()
	mock.ReadyError = errors.New("command not found")

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

func TestPoll_CanceledContext(t *testing.T) {
	mock := newMockClient()
	mock.ReadyError = context.Canceled

	cfg := config.Default()
	m := New(cfg, mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	beads, err := m.Poll(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
	if beads != nil {
		t.Errorf("expected nil beads on error, got %v", beads)
	}
}

func TestBead_JSONParsing(t *testing.T) {
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
	mock := newMockClient()
	cfg := config.Default()

	m := New(cfg, mock)
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if m.config != cfg {
		t.Error("expected config to be set")
	}
	if m.client != mock {
		t.Error("expected client to be set")
	}
	if m.history == nil {
		t.Error("expected history map to be initialized")
	}
}

func TestCalculateBackoff_FirstAttempt(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	backoff := m.calculateBackoff(1)
	if backoff != 0 {
		t.Errorf("expected 0 backoff for first attempt, got %v", backoff)
	}

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

	m := New(cfg, newMockClient())

	backoff := m.calculateBackoff(2)
	if backoff != 1*time.Minute {
		t.Errorf("expected 1m for 2 attempts, got %v", backoff)
	}

	backoff = m.calculateBackoff(3)
	if backoff != 2*time.Minute {
		t.Errorf("expected 2m for 3 attempts, got %v", backoff)
	}

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

	m := New(cfg, newMockClient())

	backoff := m.calculateBackoff(10)
	if backoff != 10*time.Minute {
		t.Errorf("expected max backoff 10m, got %v", backoff)
	}
}

func TestFilterEligible_NewBeads(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads, got %d", len(result.eligible))
	}
}

func TestFilterEligible_ExcludesEpics(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, IssueType: "task"},
		{ID: "bd-002", Priority: 2, IssueType: "epic"},
		{ID: "bd-003", Priority: 3, IssueType: "bug"},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads (excluding epic), got %d", len(result.eligible))
	}

	for _, b := range result.eligible {
		if b.IssueType == "epic" {
			t.Errorf("epic should have been filtered out, but found %s", b.ID)
		}
	}
	if result.eligible[0].ID != "bd-001" {
		t.Errorf("expected bd-001, got %s", result.eligible[0].ID)
	}
	if result.eligible[1].ID != "bd-003" {
		t.Errorf("expected bd-003, got %s", result.eligible[1].ID)
	}
}

func TestFilterEligible_ExcludesCompleted(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", result.eligible[0].ID)
	}
}

func TestFilterEligible_ExcludesAbandoned(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryAbandoned}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", result.eligible[0].ID)
	}
}

func TestFilterEligible_ExcludesInBackoff(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Hour

	m := New(cfg, newMockClient())

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

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", result.eligible[0].ID)
	}
}

func TestFilterEligible_IncludesAfterBackoff(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Millisecond

	m := New(cfg, newMockClient())

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

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads after backoff, got %d", len(result.eligible))
	}
}

func TestFilterEligible_ExcludesMaxFailures(t *testing.T) {
	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3
	cfg.Backoff.Initial = 1 * time.Millisecond

	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    3,
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 1 {
		t.Errorf("expected 1 eligible bead when max failures reached, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", result.eligible[0].ID)
	}
}

func TestNext_ReturnsHighestPriority(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-002", Title: "Low priority", Status: "open", Priority: 3, CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "bd-001", Title: "High priority", Status: "open", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead, got nil")
	}
	if bead.ID != "bd-001" {
		t.Errorf("expected highest priority bd-001, got %s", bead.ID)
	}
	if reason != ReasonSuccess {
		t.Errorf("expected ReasonSuccess, got %v", reason)
	}
}

func TestNext_SamePriorityOldestFirst(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-002", Title: "Newer", Status: "open", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)},
		{ID: "bd-001", Title: "Older", Status: "open", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, _, err := m.Next(context.Background())
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
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no work available, got %v", bead)
	}
	if reason != ReasonNoReady {
		t.Errorf("expected ReasonNoReady, got %v", reason)
	}
}

func TestNext_AllBeadsFiltered(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when all filtered, got %v", bead)
	}
	// All beads were filtered because they're completed, so ReasonNoReady is expected
	if reason != ReasonNoReady {
		t.Errorf("expected ReasonNoReady, got %v", reason)
	}
}

func TestNext_AllBeadsInBackoff_ReturnsReasonBackoff(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Hour

	m := New(cfg, mock)

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now(),
	}

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when all in backoff, got %v", bead)
	}
	if reason != ReasonBackoff {
		t.Errorf("expected ReasonBackoff, got %v", reason)
	}
}

func TestNext_AllBeadsMaxFailure_ReturnsReasonMaxFailure(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3
	cfg.Backoff.Initial = 1 * time.Millisecond

	m := New(cfg, mock)

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    3,
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when all max failures, got %v", bead)
	}
	if reason != ReasonMaxFailure {
		t.Errorf("expected ReasonMaxFailure, got %v", reason)
	}
}

func TestNext_MixedBackoffAndMaxFailure_ReturnsReasonBackoff(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
		{ID: "bd-002", Title: "Test bead 2", Status: "open", Priority: 2},
	}

	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3
	cfg.Backoff.Initial = 1 * time.Hour

	m := New(cfg, mock)

	// One bead in backoff
	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now(),
	}

	// One bead at max failures (past backoff period)
	m.history["bd-002"] = &BeadHistory{
		ID:          "bd-002",
		Status:      HistoryFailed,
		Attempts:    3,
		LastAttempt: time.Now().Add(-24 * time.Hour),
	}

	bead, reason, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead, got %v", bead)
	}
	// When there's a mix of backoff and max failures, we return backoff
	if reason != ReasonBackoff {
		t.Errorf("expected ReasonBackoff when mixed, got %v", reason)
	}
}

func TestNext_MarksAsWorking(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, _, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

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
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

	cfg := config.Default()
	cfg.Backoff.Initial = 1 * time.Millisecond

	m := New(cfg, mock)

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryFailed,
		Attempts:    1,
		LastAttempt: time.Now().Add(-1 * time.Hour),
	}

	bead, _, err := m.Next(context.Background())
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

func TestRecordSuccess(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

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
	m := New(cfg, newMockClient())

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
	m := New(cfg, newMockClient())

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

	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 3,
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

	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 2,
	}

	m.RecordFailure("bd-001", errors.New("failed"))

	h := m.history["bd-001"]
	if h.Status != HistoryFailed {
		t.Errorf("expected status failed when below max, got %s", h.Status)
	}
}

func TestStats_Empty(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

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

	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}
	m.history["bd-002"] = &BeadHistory{ID: "bd-002", Status: HistoryCompleted}
	m.history["bd-003"] = &BeadHistory{
		ID:          "bd-003",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now(),
	}
	m.history["bd-004"] = &BeadHistory{
		ID:          "bd-004",
		Status:      HistoryFailed,
		Attempts:    2,
		LastAttempt: time.Now().Add(-2 * time.Hour),
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

func TestHistory_ReturnsCopy(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistoryCompleted}

	h := m.History()
	if len(h) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(h))
	}

	h["bd-002"] = &BeadHistory{ID: "bd-002", Status: HistoryFailed}
	if len(m.history) != 1 {
		t.Errorf("internal history should still have 1 entry, got %d", len(m.history))
	}

	h["bd-001"].Status = HistoryFailed
	if m.history["bd-001"].Status != HistoryCompleted {
		t.Errorf("internal status should still be completed")
	}
}

func TestSetHistory_RestoresState(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

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
	m := New(cfg, newMockClient())

	input := map[string]*BeadHistory{
		"bd-001": {ID: "bd-001", Status: HistoryCompleted},
	}

	m.SetHistory(input)

	input["bd-001"].Status = HistoryFailed
	if m.history["bd-001"].Status != HistoryCompleted {
		t.Errorf("internal status should still be completed after input modification")
	}
}

func TestPoll_WithUnassignedFilter(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

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

	if len(mock.ReadyCalls) != 1 {
		t.Fatalf("expected 1 Ready call, got %d", len(mock.ReadyCalls))
	}
	opts := mock.ReadyCalls[0]
	if opts == nil || !opts.UnassignedOnly {
		t.Error("expected UnassignedOnly to be true")
	}
}

func TestPoll_WithLabelAndUnassigned(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
	}

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

	opts := mock.ReadyCalls[0]
	if opts == nil || opts.Label != "automated" || !opts.UnassignedOnly {
		t.Error("expected both label and unassigned options")
	}
}

func TestFilterEligible_ExcludesLabels(t *testing.T) {
	cfg := config.Default()
	cfg.WorkQueue.ExcludeLabels = []string{"manual", "blocked"}
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, Labels: []string{"automated"}},
		{ID: "bd-002", Priority: 2, Labels: []string{"manual"}},
		{ID: "bd-003", Priority: 3, Labels: []string{"urgent", "blocked"}},
		{ID: "bd-004", Priority: 4, Labels: nil},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads, got %d", len(result.eligible))
	}

	ids := make(map[string]bool)
	for _, b := range result.eligible {
		ids[b.ID] = true
	}
	if !ids["bd-001"] {
		t.Error("expected bd-001 to be eligible")
	}
	if !ids["bd-004"] {
		t.Error("expected bd-004 to be eligible")
	}
}

func TestFilterEligible_NoExcludeLabels(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "bd-001", Priority: 1, Labels: []string{"manual"}},
		{ID: "bd-002", Priority: 2, Labels: []string{"blocked"}},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads when no exclude labels, got %d", len(result.eligible))
	}
}

func TestHasExcludedLabel(t *testing.T) {
	cfg := config.Default()
	cfg.WorkQueue.ExcludeLabels = []string{"manual", "blocked"}
	m := New(cfg, newMockClient())

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
		{"case sensitive", []string{"MANUAL"}, false},
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
	m := New(cfg, newMockClient())

	if m.hasExcludedLabel([]string{"manual", "blocked"}) {
		t.Error("expected false when exclude list is empty")
	}
}

func TestBuildDescendantSet_SingleLevel(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "epic-001"},
		{ID: "task-003", Parent: "other-epic"},
	}

	descendants := buildDescendantSet("epic-001", beads)

	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if !descendants["task-001"] {
		t.Error("expected task-001 to be in descendants")
	}
	if !descendants["task-002"] {
		t.Error("expected task-002 to be in descendants")
	}
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

	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if len(descendants) != 1 {
		t.Errorf("expected 1 descendant (epic only), got %d", len(descendants))
	}
}

func TestBuildDescendantSet_EmptyBeads(t *testing.T) {
	descendants := buildDescendantSet("epic-001", nil)

	if !descendants["epic-001"] {
		t.Error("expected epic-001 to be in descendants")
	}
	if len(descendants) != 1 {
		t.Errorf("expected 1 descendant, got %d", len(descendants))
	}
}

func TestFilterEligible_WithEpicDescendants(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
		{ID: "task-003", Priority: 3, IssueType: "task"},
	}

	descendants := map[string]bool{
		"epic-001": true,
		"task-001": true,
		"task-003": true,
	}

	result := m.filterEligible(beads, descendants)

	if len(result.eligible) != 2 {
		t.Fatalf("expected 2 eligible beads, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "task-001" {
		t.Errorf("expected task-001, got %s", result.eligible[0].ID)
	}
	if result.eligible[1].ID != "task-003" {
		t.Errorf("expected task-003, got %s", result.eligible[1].ID)
	}
}

func TestFilterEligible_WithNilDescendants(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
	}

	result := m.filterEligible(beads, nil)

	if len(result.eligible) != 2 {
		t.Errorf("expected 2 eligible beads when no epic filter, got %d", len(result.eligible))
	}
}

func TestFilterEligible_EpicDescendantsEmptyResult(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	beads := []Bead{
		{ID: "task-001", Priority: 1, IssueType: "task"},
		{ID: "task-002", Priority: 2, IssueType: "task"},
	}

	descendants := map[string]bool{
		"epic-001": true,
	}

	result := m.filterEligible(beads, descendants)

	if len(result.eligible) != 0 {
		t.Errorf("expected 0 eligible beads, got %d", len(result.eligible))
	}
}

func TestNext_WithEpicFilter(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", Title: "Task in epic", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "task-002", Title: "Task outside epic", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", Title: "Epic", Status: "open", IssueType: "epic"},
		{ID: "task-001", Title: "Task in epic", Status: "open", Parent: "epic-001"},
		{ID: "task-002", Title: "Task outside epic", Status: "open", Parent: "other-epic"},
	}

	cfg := config.Default()
	cfg.WorkQueue.Epic = "epic-001"
	m := New(cfg, mock)

	bead, _, err := m.Next(context.Background())
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
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-002", Title: "Task outside epic", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", Title: "Epic", Status: "open", IssueType: "epic"},
		{ID: "task-002", Title: "Task outside epic", Status: "open", Parent: "other-epic"},
	}

	cfg := config.Default()
	cfg.WorkQueue.Epic = "epic-001"
	m := New(cfg, mock)

	bead, _, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no eligible beads in epic, got %v", bead)
	}
}

func TestNext_WithoutEpicFilter(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{
		{ID: "bd-001", Title: "Test bead 1", Status: "open", Priority: 1},
		{ID: "bd-002", Title: "Test bead 2", Status: "open", Priority: 2},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, _, err := m.Next(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead when no epic filter")
	}
}

func TestIdentifyTopLevelItems_EpicsAndRoots(t *testing.T) {
	beads := []Bead{
		{ID: "epic-001", IssueType: "epic", Priority: 2, CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "task-001", Parent: "epic-001", Priority: 1},
		{ID: "standalone-001", IssueType: "task", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "epic-002", IssueType: "epic", Priority: 3, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
	}

	topLevel := identifyTopLevelItems(beads)

	if len(topLevel) != 3 {
		t.Fatalf("expected 3 top-level items, got %d", len(topLevel))
	}

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
		{ID: "epic-001", IssueType: "epic"},
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
		{ID: "standalone-001", IssueType: "task"},
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
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "epic-002"},
	}

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
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-epic1", Title: "Task in epic1", Status: "open", Priority: 2, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		{ID: "task-epic2", Title: "Task in epic2", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", Title: "Epic 1", Status: "open", IssueType: "epic", Priority: 2},
		{ID: "epic-002", Title: "Epic 2", Status: "open", IssueType: "epic", Priority: 1},
		{ID: "task-epic1", Title: "Task in epic1", Status: "open", Parent: "epic-001"},
		{ID: "task-epic2", Title: "Task in epic2", Status: "open", Parent: "epic-002"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	m.SetActiveTopLevel("epic-001")

	bead, reason, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	if bead.ID != "task-epic1" {
		t.Errorf("expected task-epic1 (from active top-level), got %s", bead.ID)
	}

	if m.ActiveTopLevel() != "epic-001" {
		t.Errorf("expected active top-level to remain epic-001, got %s", m.ActiveTopLevel())
	}

	if reason != ReasonSuccess {
		t.Errorf("expected ReasonSuccess, got %v", reason)
	}
}

func TestNextTopLevel_SwitchesWhenExhausted(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-epic2", Title: "Task in epic2", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", Title: "Epic 1", Status: "open", IssueType: "epic", Priority: 2},
		{ID: "epic-002", Title: "Epic 2", Status: "open", IssueType: "epic", Priority: 1},
		{ID: "task-epic2", Title: "Task in epic2", Status: "open", Parent: "epic-002"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	m.SetActiveTopLevel("epic-001")

	bead, _, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected a bead")
	}

	if bead.ID != "task-epic2" {
		t.Errorf("expected task-epic2, got %s", bead.ID)
	}

	if m.ActiveTopLevel() != "epic-002" {
		t.Errorf("expected active top-level to switch to epic-002, got %s", m.ActiveTopLevel())
	}
}

func TestNextTopLevel_NoBeads(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{}

	cfg := config.Default()
	m := New(cfg, mock)

	bead, reason, err := m.NextTopLevel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("expected nil bead when no work available, got %v", bead)
	}
	if reason != ReasonNoReady {
		t.Errorf("expected ReasonNoReady, got %v", reason)
	}
}

func TestActiveTopLevel_GetSet(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	if m.ActiveTopLevel() != "" {
		t.Errorf("expected empty active top-level, got %s", m.ActiveTopLevel())
	}

	m.SetActiveTopLevel("epic-001")
	if m.ActiveTopLevel() != "epic-001" {
		t.Errorf("expected epic-001, got %s", m.ActiveTopLevel())
	}

	m.ClearActiveTopLevel()
	if m.ActiveTopLevel() != "" {
		t.Errorf("expected empty after clear, got %s", m.ActiveTopLevel())
	}
}

func TestFetchAllBeads(t *testing.T) {
	mock := newMockClient()
	mock.ListResponse = []brclient.Bead{
		{ID: "bead-001", Title: "Bead 1", Status: "open"},
		{ID: "bead-002", Title: "Bead 2", Status: "open"},
	}

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
	mock := newMockClient()
	mock.ListResponse = nil

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
	mock := newMockClient()
	mock.ListError = errors.New("command failed")

	cfg := config.Default()
	m := New(cfg, mock)

	_, err := m.fetchAllBeads(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterEligible_ExcludesSkipped(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{ID: "bd-001", Status: HistorySkipped}

	beads := []Bead{
		{ID: "bd-001", Priority: 1},
		{ID: "bd-002", Priority: 2},
	}

	result := m.filterEligible(beads, nil)
	if len(result.eligible) != 1 {
		t.Errorf("expected 1 eligible bead, got %d", len(result.eligible))
	}
	if result.eligible[0].ID != "bd-002" {
		t.Errorf("expected bd-002, got %s", result.eligible[0].ID)
	}
}

func TestRecordSkipped(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.RecordSkipped("bd-001")

	h := m.history["bd-001"]
	if h == nil {
		t.Fatal("expected history entry")
	}
	if h.Status != HistorySkipped {
		t.Errorf("expected status skipped, got %s", h.Status)
	}
}

func TestRecordSkipped_ExistingHistory(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:       "bd-001",
		Status:   HistoryWorking,
		Attempts: 2,
	}

	m.RecordSkipped("bd-001")

	h := m.history["bd-001"]
	if h.Status != HistorySkipped {
		t.Errorf("expected status skipped, got %s", h.Status)
	}
	if h.Attempts != 2 {
		t.Errorf("expected attempts preserved as 2, got %d", h.Attempts)
	}
}

func TestResetBead(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistorySkipped,
		Attempts:    3,
		LastAttempt: time.Now(),
		LastError:   "test error",
	}

	m.ResetBead("bd-001")

	h := m.history["bd-001"]
	if h == nil {
		t.Fatal("expected history entry to still exist")
	}
	if h.Status != HistoryPending {
		t.Errorf("expected status pending, got %s", h.Status)
	}
	if h.Attempts != 0 {
		t.Errorf("expected attempts 0, got %d", h.Attempts)
	}
	if !h.LastAttempt.IsZero() {
		t.Errorf("expected LastAttempt to be zero, got %v", h.LastAttempt)
	}
	if h.LastError != "" {
		t.Errorf("expected LastError to be empty, got %s", h.LastError)
	}
}

func TestResetBead_NoExistingHistory(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.ResetBead("bd-001")

	if m.history["bd-001"] != nil {
		t.Error("expected no history entry for non-existent bead")
	}
}

func TestResetBead_AbandonedBead(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, newMockClient())

	m.history["bd-001"] = &BeadHistory{
		ID:          "bd-001",
		Status:      HistoryAbandoned,
		Attempts:    5,
		LastAttempt: time.Now(),
		LastError:   "max failures exceeded",
	}

	m.ResetBead("bd-001")

	h := m.history["bd-001"]
	if h.Status != HistoryPending {
		t.Errorf("expected status pending after reset, got %s", h.Status)
	}
	if h.Attempts != 0 {
		t.Errorf("expected attempts 0 after reset, got %d", h.Attempts)
	}
}

func TestHasEligibleReadyDescendants_Basic(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
		{ID: "task-002", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "other-epic"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasEligible {
		t.Error("expected epic-001 to have eligible ready descendants")
	}
}

func TestHasEligibleReadyDescendants_NoReadyBeads(t *testing.T) {
	mock := newMockClient()
	mock.ReadyResponse = []brclient.Bead{}

	cfg := config.Default()
	m := New(cfg, mock)

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when no ready beads")
	}
}

func TestHasEligibleReadyDescendants_NoDescendants(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-other", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-other", Parent: "other-epic"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when no ready descendants")
	}
}

func TestHasEligibleReadyDescendants_AllAbandoned(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	// Mark the only descendant as abandoned
	m.history["task-001"] = &BeadHistory{ID: "task-001", Status: HistoryAbandoned}

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when all descendants are abandoned")
	}
}

func TestHasEligibleReadyDescendants_AllSkipped(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	// Mark the only descendant as skipped
	m.history["task-001"] = &BeadHistory{ID: "task-001", Status: HistorySkipped}

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when all descendants are skipped")
	}
}

func TestHasEligibleReadyDescendants_AllMaxFailures(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
	}

	cfg := config.Default()
	cfg.Backoff.MaxFailures = 3
	cfg.Backoff.Initial = 1 * time.Millisecond
	m := New(cfg, mock)

	// Mark the only descendant at max failures
	m.history["task-001"] = &BeadHistory{
		ID:          "task-001",
		Status:      HistoryFailed,
		Attempts:    3,
		LastAttempt: time.Now().Add(-1 * time.Hour), // Past backoff
	}

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when all descendants hit max failures")
	}
}

func TestHasEligibleReadyDescendants_SomeEligible(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
		{ID: "task-002", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "task-001", Parent: "epic-001"},
		{ID: "task-002", Parent: "epic-001"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	// Mark one descendant as abandoned, but leave the other eligible
	m.history["task-001"] = &BeadHistory{ID: "task-001", Status: HistoryAbandoned}

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasEligible {
		t.Error("expected true when at least one descendant is eligible")
	}
}

func TestHasEligibleReadyDescendants_SkipsEpics(t *testing.T) {
	mock := newMockClient()

	// The only ready bead is an epic
	mock.ReadyResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEligible {
		t.Error("expected false when only ready bead is an epic")
	}
}

func TestHasEligibleReadyDescendants_NestedDescendants(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "nested-task", IssueType: "task"},
	}

	mock.ListResponse = []brclient.Bead{
		{ID: "epic-001", IssueType: "epic"},
		{ID: "sub-epic", IssueType: "epic", Parent: "epic-001"},
		{ID: "nested-task", Parent: "sub-epic"},
	}

	cfg := config.Default()
	m := New(cfg, mock)

	hasEligible, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasEligible {
		t.Error("expected true for nested descendants")
	}
}

func TestHasEligibleReadyDescendants_PollError(t *testing.T) {
	mock := newMockClient()
	mock.ReadyError = errors.New("connection refused")

	cfg := config.Default()
	m := New(cfg, mock)

	_, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err == nil {
		t.Error("expected error when poll fails")
	}
}

func TestHasEligibleReadyDescendants_ListError(t *testing.T) {
	mock := newMockClient()

	mock.ReadyResponse = []brclient.Bead{
		{ID: "task-001", IssueType: "task"},
	}
	mock.ListError = errors.New("connection refused")

	cfg := config.Default()
	m := New(cfg, mock)

	_, err := m.HasEligibleReadyDescendants(context.Background(), "epic-001")
	if err == nil {
		t.Error("expected error when list fails")
	}
}
