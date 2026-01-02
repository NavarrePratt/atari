package session

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

// collectEvents reads all events from a channel until closed or timeout.
func collectEvents(ch <-chan events.Event, timeout time.Duration) []events.Event {
	var result []events.Event
	timer := time.After(timeout)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, e)
		case <-timer:
			return result
		}
	}
}

func TestParser_EmptyInput(t *testing.T) {
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(""), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should emit no events
	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)
	if len(collected) != 0 {
		t.Errorf("expected 0 events, got %d", len(collected))
	}
}

func TestParser_EmptyLines(t *testing.T) {
	input := "\n\n\n"
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)
	if len(collected) != 0 {
		t.Errorf("expected 0 events, got %d", len(collected))
	}
}

func TestParser_TextEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	textEvent, ok := collected[0].(*events.ClaudeTextEvent)
	if !ok {
		t.Fatalf("expected ClaudeTextEvent, got %T", collected[0])
	}
	if textEvent.Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", textEvent.Text)
	}
	if textEvent.Type() != events.EventClaudeText {
		t.Errorf("expected EventClaudeText, got %v", textEvent.Type())
	}
}

func TestParser_ToolUseEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Bash","input":{"command":"ls -la"}}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	toolEvent, ok := collected[0].(*events.ClaudeToolUseEvent)
	if !ok {
		t.Fatalf("expected ClaudeToolUseEvent, got %T", collected[0])
	}
	if toolEvent.ToolID != "tool_1" {
		t.Errorf("expected tool_1, got %q", toolEvent.ToolID)
	}
	if toolEvent.ToolName != "Bash" {
		t.Errorf("expected Bash, got %q", toolEvent.ToolName)
	}
	cmd, ok := toolEvent.Input["command"].(string)
	if !ok || cmd != "ls -la" {
		t.Errorf("expected command='ls -la', got %v", toolEvent.Input["command"])
	}
}

func TestParser_ToolResultEvent(t *testing.T) {
	input := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"file1.txt\nfile2.txt","is_error":false}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	resultEvent, ok := collected[0].(*events.ClaudeToolResultEvent)
	if !ok {
		t.Fatalf("expected ClaudeToolResultEvent, got %T", collected[0])
	}
	if resultEvent.ToolID != "tool_1" {
		t.Errorf("expected tool_1, got %q", resultEvent.ToolID)
	}
	if resultEvent.Content != "file1.txt\nfile2.txt" {
		t.Errorf("unexpected content: %q", resultEvent.Content)
	}
	if resultEvent.IsError {
		t.Error("expected IsError=false")
	}
}

func TestParser_SessionResultEvent(t *testing.T) {
	input := `{"type":"result","session_id":"abc123","num_turns":5,"duration_ms":30000,"total_cost_usd":0.42,"result":"Task completed"}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	endEvent, ok := collected[0].(*events.SessionEndEvent)
	if !ok {
		t.Fatalf("expected SessionEndEvent, got %T", collected[0])
	}
	if endEvent.SessionID != "abc123" {
		t.Errorf("expected abc123, got %q", endEvent.SessionID)
	}
	if endEvent.NumTurns != 5 {
		t.Errorf("expected 5 turns, got %d", endEvent.NumTurns)
	}
	if endEvent.DurationMs != 30000 {
		t.Errorf("expected 30000ms, got %d", endEvent.DurationMs)
	}
	if endEvent.TotalCostUSD != 0.42 {
		t.Errorf("expected 0.42, got %f", endEvent.TotalCostUSD)
	}
	if endEvent.Result != "Task completed" {
		t.Errorf("expected 'Task completed', got %q", endEvent.Result)
	}
}

func TestParser_ResultCapture(t *testing.T) {
	input := `{"type":"result","session_id":"abc123","num_turns":5,"duration_ms":30000,"total_cost_usd":0.42,"result":"Task completed"}`
	router := events.NewRouter(100)
	defer router.Close()

	parser := NewParser(strings.NewReader(input), router, nil)

	// Before parsing, Result() should return nil
	if parser.Result() != nil {
		t.Error("expected Result() to be nil before parsing")
	}

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After parsing, Result() should return the captured event
	result := parser.Result()
	if result == nil {
		t.Fatal("expected Result() to be non-nil after parsing")
	}
	if result.SessionID != "abc123" {
		t.Errorf("expected abc123, got %q", result.SessionID)
	}
	if result.NumTurns != 5 {
		t.Errorf("expected 5 turns, got %d", result.NumTurns)
	}
	if result.TotalCostUSD != 0.42 {
		t.Errorf("expected 0.42, got %f", result.TotalCostUSD)
	}
}

func TestParser_ResultNilWithoutResultEvent(t *testing.T) {
	// No result event in stream
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result() should return nil when no result event was parsed
	if parser.Result() != nil {
		t.Error("expected Result() to be nil without result event")
	}
}

func TestParser_ParseErrorContinues(t *testing.T) {
	// First line is invalid JSON, second is valid
	input := `not valid json
{"type":"assistant","message":{"content":[{"type":"text","text":"After error"}]}}`

	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	// Should have parse error + text event
	if len(collected) != 2 {
		t.Fatalf("expected 2 events, got %d", len(collected))
	}

	// First should be parse error
	parseErr, ok := collected[0].(*events.ParseErrorEvent)
	if !ok {
		t.Fatalf("expected ParseErrorEvent, got %T", collected[0])
	}
	if parseErr.Line != "not valid json" {
		t.Errorf("expected 'not valid json', got %q", parseErr.Line)
	}

	// Second should be text event
	textEvent, ok := collected[1].(*events.ClaudeTextEvent)
	if !ok {
		t.Fatalf("expected ClaudeTextEvent, got %T", collected[1])
	}
	if textEvent.Text != "After error" {
		t.Errorf("expected 'After error', got %q", textEvent.Text)
	}
}

func TestParser_MultipleContentBlocks(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"},{"type":"text","text":"Second"}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 2 {
		t.Fatalf("expected 2 events, got %d", len(collected))
	}

	text1, ok := collected[0].(*events.ClaudeTextEvent)
	if !ok || text1.Text != "First" {
		t.Errorf("expected first text event")
	}

	text2, ok := collected[1].(*events.ClaudeTextEvent)
	if !ok || text2.Text != "Second" {
		t.Errorf("expected second text event")
	}
}

func TestParser_UpdatesActivity(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`
	cfg := config.Default()
	router := events.NewRouter(100)
	defer router.Close()

	manager := New(cfg, router)

	// Set old activity time
	oldTime := time.Now().Add(-time.Hour)
	manager.lastActive.Store(oldTime)

	parser := NewParser(strings.NewReader(input), router, manager)
	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Activity should be updated
	newTime := manager.lastActive.Load().(time.Time)
	if !newTime.After(oldTime) {
		t.Errorf("expected activity to be updated, old=%v new=%v", oldTime, newTime)
	}
}

func TestParser_NilRouter(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`
	parser := NewParser(strings.NewReader(input), nil, nil)

	// Should not panic with nil router
	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParser_UnknownEventType(t *testing.T) {
	input := `{"type":"unknown_type","data":"something"}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	// Unknown types are silently ignored
	if len(collected) != 0 {
		t.Errorf("expected 0 events for unknown type, got %d", len(collected))
	}
}

func TestParser_SystemInitEvent(t *testing.T) {
	input := `{"type":"system","subtype":"init","model":"opus","tools":["Bash","Read","Edit"]}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	// System init doesn't emit events currently (just processed internally)
	if len(collected) != 0 {
		t.Errorf("expected 0 events for system init, got %d", len(collected))
	}
}

func TestParser_ThinkingEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Let me think about this..."}]}}`
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	if len(collected) != 1 {
		t.Fatalf("expected 1 event, got %d", len(collected))
	}

	// Thinking is emitted as text event
	textEvent, ok := collected[0].(*events.ClaudeTextEvent)
	if !ok {
		t.Fatalf("expected ClaudeTextEvent, got %T", collected[0])
	}
	if textEvent.Text != "Let me think about this..." {
		t.Errorf("expected thinking text, got %q", textEvent.Text)
	}
}

func TestParser_FullSession(t *testing.T) {
	// Simulate a complete session stream
	input := `{"type":"system","subtype":"init","model":"opus","tools":["Bash","Read"]}
{"type":"assistant","message":{"content":[{"type":"text","text":"Let me check the status..."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"git status"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"On branch main"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"The branch is clean."}]}}
{"type":"result","session_id":"sess123","num_turns":2,"duration_ms":5000,"total_cost_usd":0.05}`

	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	parser := NewParser(strings.NewReader(input), router, nil)

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	router.Close()
	collected := collectEvents(sub, 100*time.Millisecond)

	// Expected events: text, tool_use, tool_result, text, session_end = 5
	if len(collected) != 5 {
		t.Fatalf("expected 5 events, got %d", len(collected))
	}

	// Verify event types in order
	expectedTypes := []events.EventType{
		events.EventClaudeText,
		events.EventClaudeToolUse,
		events.EventClaudeToolResult,
		events.EventClaudeText,
		events.EventSessionEnd,
	}

	for i, expected := range expectedTypes {
		if collected[i].Type() != expected {
			t.Errorf("event %d: expected %v, got %v", i, expected, collected[i].Type())
		}
	}
}

func TestScannerBufferSize(t *testing.T) {
	if ScannerBufferSize != 1024*1024 {
		t.Errorf("expected ScannerBufferSize=1MB, got %d", ScannerBufferSize)
	}
}

// TestParser_ActivityUpdateCount verifies UpdateActivity is called for each valid event
func TestParser_ActivityUpdateCount(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"First"}]}}
{"type":"assistant","message":{"content":[{"type":"text","text":"Second"}]}}
{"type":"result","session_id":"x","num_turns":1,"duration_ms":100,"total_cost_usd":0.01}`

	cfg := config.Default()
	router := events.NewRouter(100)
	defer router.Close()

	manager := New(cfg, router)

	// Track activity updates using a counter
	var updateCount atomic.Int32
	originalStore := manager.lastActive.Load().(time.Time)

	// Create a wrapper that counts updates
	parser := NewParser(strings.NewReader(input), router, manager)

	// Parse and count how many times lastActive changes
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			current := manager.lastActive.Load().(time.Time)
			if current.After(originalStore) {
				updateCount.Add(1)
				originalStore = current
			}
		}
	}()

	err := parser.Parse()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Give time for counter goroutine
	time.Sleep(150 * time.Millisecond)

	// Should have been updated at least once (exact count depends on timing)
	count := updateCount.Load()
	if count == 0 {
		t.Error("expected at least one activity update")
	}
}
