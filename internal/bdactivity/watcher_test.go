package bdactivity

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

func testConfig() *config.BDActivityConfig {
	return &config.BDActivityConfig{
		Enabled:           true,
		ReconnectDelay:    10 * time.Millisecond,
		MaxReconnectDelay: 50 * time.Millisecond,
	}
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create beads directory: %v", err)
	}
	return dir
}

func writeJSONLFile(t *testing.T, path string, beads []map[string]interface{}) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create JSONL file: %v", err)
	}
	defer func() { _ = f.Close() }()

	for _, bead := range beads {
		data, err := json.Marshal(bead)
		if err != nil {
			t.Fatalf("failed to marshal bead: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatalf("failed to write bead: %v", err)
		}
		if _, err := f.WriteString("\n"); err != nil {
			t.Fatalf("failed to write newline: %v", err)
		}
	}
}

func appendJSONLFile(t *testing.T, path string, bead map[string]interface{}) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("failed to open JSONL file: %v", err)
	}
	defer func() { _ = f.Close() }()

	data, err := json.Marshal(bead)
	if err != nil {
		t.Fatalf("failed to marshal bead: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("failed to write bead: %v", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		t.Fatalf("failed to write newline: %v", err)
	}
}

func TestWatcher_StartStop(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !watcher.Running() {
		t.Error("expected Running() to be true after Start")
	}

	time.Sleep(50 * time.Millisecond)

	err = watcher.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if watcher.Running() {
		t.Error("expected Running() to be false after Stop")
	}
}

func TestWatcher_DoubleStart(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	err = watcher.Start(ctx)
	if err == nil {
		t.Error("expected second Start to fail")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatcher_StopIdempotent(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		err = watcher.Stop()
		if err != nil {
			t.Errorf("Stop %d failed: %v", i, err)
		}
	}
}

func TestWatcher_StopBeforeStart(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	err := watcher.Stop()
	if err != nil {
		t.Errorf("Stop before Start should not error: %v", err)
	}
}

func TestWatcher_InitialLoad(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Test issue 1", "status": "open", "priority": 1, "issue_type": "task"},
		{"id": "bd-002", "title": "Test issue 2", "status": "in_progress", "priority": 2, "issue_type": "bug"},
	})

	router := events.NewRouter(20)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Should get 2 BeadChangedEvents for initial load
	var received []*events.BeadChangedEvent
	timeout := time.After(500 * time.Millisecond)
	for len(received) < 2 {
		select {
		case e := <-sub:
			if changed, ok := e.(*events.BeadChangedEvent); ok {
				received = append(received, changed)
			}
		case <-timeout:
			t.Fatalf("timeout waiting for events, got %d", len(received))
		}
	}

	// Verify events
	ids := make(map[string]bool)
	for _, e := range received {
		ids[e.BeadID] = true
		if e.OldState != nil {
			t.Errorf("expected OldState to be nil for initial load, got %+v", e.OldState)
		}
		if e.NewState == nil {
			t.Error("expected NewState to be non-nil for initial load")
		}
	}
	if !ids["bd-001"] || !ids["bd-002"] {
		t.Errorf("expected events for bd-001 and bd-002, got %v", ids)
	}
}

func TestWatcher_FileChange(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Test issue", "status": "open", "priority": 1, "issue_type": "task"},
	})

	router := events.NewRouter(20)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Wait for initial load event
	timeout := time.After(500 * time.Millisecond)
	select {
	case <-sub:
		// Got initial event
	case <-timeout:
		t.Fatal("timeout waiting for initial event")
	}

	// Add a new bead
	time.Sleep(150 * time.Millisecond) // Wait for debounce
	appendJSONLFile(t, jsonlPath, map[string]interface{}{
		"id": "bd-002", "title": "New issue", "status": "open", "priority": 2, "issue_type": "task",
	})

	// Wait for change event
	timeout = time.After(500 * time.Millisecond)
	select {
	case e := <-sub:
		changed, ok := e.(*events.BeadChangedEvent)
		if !ok {
			t.Fatalf("expected BeadChangedEvent, got %T", e)
		}
		if changed.BeadID != "bd-002" {
			t.Errorf("expected bead_id bd-002, got %s", changed.BeadID)
		}
		if changed.OldState != nil {
			t.Error("expected OldState to be nil for new bead")
		}
		if changed.NewState == nil || changed.NewState.Title != "New issue" {
			t.Errorf("unexpected NewState: %+v", changed.NewState)
		}
	case <-timeout:
		t.Fatal("timeout waiting for change event")
	}
}

func TestWatcher_FileModification(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Test issue", "status": "open", "priority": 1, "issue_type": "task"},
	})

	router := events.NewRouter(20)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Wait for initial load
	timeout := time.After(500 * time.Millisecond)
	select {
	case <-sub:
	case <-timeout:
		t.Fatal("timeout waiting for initial event")
	}

	// Modify the bead status
	time.Sleep(150 * time.Millisecond)
	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Test issue", "status": "in_progress", "priority": 1, "issue_type": "task"},
	})

	// Wait for change event
	timeout = time.After(500 * time.Millisecond)
	select {
	case e := <-sub:
		changed, ok := e.(*events.BeadChangedEvent)
		if !ok {
			t.Fatalf("expected BeadChangedEvent, got %T", e)
		}
		if changed.BeadID != "bd-001" {
			t.Errorf("expected bead_id bd-001, got %s", changed.BeadID)
		}
		if changed.OldState == nil || changed.OldState.Status != "open" {
			t.Errorf("expected OldState.Status to be 'open', got %+v", changed.OldState)
		}
		if changed.NewState == nil || changed.NewState.Status != "in_progress" {
			t.Errorf("expected NewState.Status to be 'in_progress', got %+v", changed.NewState)
		}
	case <-timeout:
		t.Fatal("timeout waiting for change event")
	}
}

func TestWatcher_FileTruncation(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Issue 1", "status": "open", "priority": 1, "issue_type": "task"},
		{"id": "bd-002", "title": "Issue 2", "status": "open", "priority": 2, "issue_type": "task"},
	})

	router := events.NewRouter(20)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Drain initial events
	timeout := time.After(500 * time.Millisecond)
	drained := 0
drainLoop:
	for drained < 2 {
		select {
		case <-sub:
			drained++
		case <-timeout:
			break drainLoop
		}
	}

	// Truncate file (simulate br sync)
	time.Sleep(150 * time.Millisecond)
	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "Issue 1", "status": "closed", "priority": 1, "issue_type": "task"},
	})

	// Should get events for modified bd-001 and deleted bd-002
	timeout = time.After(500 * time.Millisecond)
	var changes []*events.BeadChangedEvent
collectLoop:
	for len(changes) < 2 {
		select {
		case e := <-sub:
			if changed, ok := e.(*events.BeadChangedEvent); ok {
				changes = append(changes, changed)
			}
		case <-timeout:
			break collectLoop
		}
	}

	if len(changes) < 2 {
		t.Fatalf("expected 2 change events, got %d", len(changes))
	}

	// Check for the modification and deletion
	var gotModify, gotDelete bool
	for _, c := range changes {
		if c.BeadID == "bd-001" && c.NewState != nil && c.NewState.Status == "closed" {
			gotModify = true
		}
		if c.BeadID == "bd-002" && c.NewState == nil {
			gotDelete = true
		}
	}
	if !gotModify {
		t.Error("expected modification event for bd-001")
	}
	if !gotDelete {
		t.Error("expected deletion event for bd-002")
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	ctx, cancel := context.WithCancel(context.Background())

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	cancel()

	timeout := time.After(200 * time.Millisecond)
	for watcher.Running() {
		select {
		case <-timeout:
			_ = watcher.Stop()
			t.Fatal("watcher did not stop after context cancel")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcher_RunningState(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	if watcher.Running() {
		t.Error("expected Running() to be false before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !watcher.Running() {
		t.Error("expected Running() to be true after Start")
	}

	time.Sleep(50 * time.Millisecond)
	_ = watcher.Stop()

	if watcher.Running() {
		t.Error("expected Running() to be false after Stop")
	}
}

func TestWatcher_FileNotExist(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")
	// Do not create the file

	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should succeed even if file doesn't exist
	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	if !watcher.Running() {
		t.Error("expected watcher to be running")
	}

	time.Sleep(50 * time.Millisecond)

	// Watcher should still be running
	if !watcher.Running() {
		t.Error("watcher should still be running even if file doesn't exist")
	}
}

func TestWatcher_FileCreatedLater(t *testing.T) {
	dir := setupTestDir(t)
	jsonlPath := filepath.Join(dir, ".beads", "issues.jsonl")
	// Do not create the file yet

	router := events.NewRouter(20)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := NewWithPath(testConfig(), router, logger, jsonlPath)

	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Create file after watcher started
	time.Sleep(100 * time.Millisecond)
	writeJSONLFile(t, jsonlPath, []map[string]interface{}{
		{"id": "bd-001", "title": "New issue", "status": "open", "priority": 1, "issue_type": "task"},
	})

	// Should detect the file creation
	timeout := time.After(500 * time.Millisecond)
	select {
	case e := <-sub:
		changed, ok := e.(*events.BeadChangedEvent)
		if !ok {
			t.Fatalf("expected BeadChangedEvent, got %T", e)
		}
		if changed.BeadID != "bd-001" {
			t.Errorf("expected bead_id bd-001, got %s", changed.BeadID)
		}
	case <-timeout:
		t.Fatal("timeout waiting for event after file creation")
	}
}

func TestParseJSONLLine(t *testing.T) {
	tests := []struct {
		name     string
		line     []byte
		wantID   string
		wantErr  bool
		wantNil  bool
	}{
		{
			name:   "valid bead",
			line:   []byte(`{"id":"bd-001","title":"Test","status":"open","priority":1,"issue_type":"task"}`),
			wantID: "bd-001",
		},
		{
			name:    "empty line",
			line:    []byte{},
			wantNil: true,
		},
		{
			name:    "invalid json",
			line:    []byte(`{invalid`),
			wantErr: true,
		},
		{
			name:    "missing id",
			line:    []byte(`{"title":"Test","status":"open"}`),
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bead, err := ParseJSONLLine(tc.line)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantNil && bead != nil {
				t.Errorf("expected nil bead, got %+v", bead)
			}
			if tc.wantID != "" {
				if bead == nil {
					t.Fatal("expected non-nil bead")
				}
				if bead.ID != tc.wantID {
					t.Errorf("expected ID %s, got %s", tc.wantID, bead.ID)
				}
			}
		})
	}
}

func TestBeadStateEqual(t *testing.T) {
	tests := []struct {
		name   string
		a, b   *events.BeadState
		expect bool
	}{
		{
			name:   "both nil",
			a:      nil,
			b:      nil,
			expect: true,
		},
		{
			name:   "one nil",
			a:      &events.BeadState{ID: "bd-001"},
			b:      nil,
			expect: false,
		},
		{
			name:   "equal",
			a:      &events.BeadState{ID: "bd-001", Title: "Test", Status: "open", Priority: 1, IssueType: "task"},
			b:      &events.BeadState{ID: "bd-001", Title: "Test", Status: "open", Priority: 1, IssueType: "task"},
			expect: true,
		},
		{
			name:   "different status",
			a:      &events.BeadState{ID: "bd-001", Status: "open"},
			b:      &events.BeadState{ID: "bd-001", Status: "closed"},
			expect: false,
		},
		{
			name:   "different title",
			a:      &events.BeadState{ID: "bd-001", Title: "Old"},
			b:      &events.BeadState{ID: "bd-001", Title: "New"},
			expect: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := beadStateEqual(tc.a, tc.b)
			if result != tc.expect {
				t.Errorf("beadStateEqual(%+v, %+v) = %v, want %v", tc.a, tc.b, result, tc.expect)
			}
		})
	}
}
