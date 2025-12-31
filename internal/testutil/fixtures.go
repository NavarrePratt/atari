package testutil

// Sample bd ready responses

// SampleBeadReadyJSON is a typical bd ready --json response with multiple beads.
var SampleBeadReadyJSON = `[
  {"id": "bd-001", "title": "Test bead 1", "description": "First test bead", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z", "created_by": "user"},
  {"id": "bd-002", "title": "Test bead 2", "description": "Second test bead", "status": "open", "priority": 2, "issue_type": "task", "created_at": "2024-01-15T11:00:00Z", "created_by": "user"}
]`

// SingleBeadReadyJSON is a bd ready response with a single bead.
var SingleBeadReadyJSON = `[
  {"id": "bd-001", "title": "Test bead 1", "description": "First test bead", "status": "open", "priority": 1, "issue_type": "task", "created_at": "2024-01-15T10:00:00Z", "created_by": "user"}
]`

// EmptyBeadReadyJSON is a bd ready response when no beads are available.
var EmptyBeadReadyJSON = `[]`

// Sample bd agent state responses

// BDAgentStateSuccess is a successful bd agent state response.
var BDAgentStateSuccess = `{"status": "ok"}`

// Sample bd close responses

// BDCloseSuccess is a successful bd close response.
var BDCloseSuccess = `{"id": "bd-001", "status": "closed"}`

// Sample Claude stream-json events

// SampleClaudeInit is the initial system message from Claude.
var SampleClaudeInit = `{"type":"system","subtype":"init","session_id":"test-session-001","cwd":"/workspace","tools":["Bash","Read","Write"]}`

// SampleClaudeAssistant is a typical assistant text response.
var SampleClaudeAssistant = `{"type":"assistant","message":{"content":[{"type":"text","text":"Working on the task..."}]}}`

// SampleClaudeToolUse is an assistant message with tool use.
var SampleClaudeToolUse = `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_001","name":"Bash","input":{"command":"echo hello"}}]}}`

// SampleClaudeToolResult is a user message with tool result.
var SampleClaudeToolResult = `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_001","content":"hello\n"}]}}`

// SampleClaudeResultSuccess is a successful session completion.
var SampleClaudeResultSuccess = `{"type":"result","subtype":"success","total_cost_usd":0.05,"duration_ms":12500,"num_turns":5,"session_id":"test-session-001"}`

// SampleClaudeResultError is a failed session due to tool error.
var SampleClaudeResultError = `{"type":"result","subtype":"error_tool_use","error":"command failed with exit code 1"}`

// SampleClaudeResultMaxTurns is a session that hit max turns limit.
var SampleClaudeResultMaxTurns = `{"type":"result","subtype":"error_max_turns","total_cost_usd":0.15,"duration_ms":60000,"num_turns":10}`

// Bead represents the structure of a bead from bd ready output.
type Bead struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    int    `json:"priority"`
	IssueType   string `json:"issue_type"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

// SampleBead1 is a test bead with priority 1.
var SampleBead1 = Bead{
	ID:          "bd-001",
	Title:       "Test bead 1",
	Description: "First test bead",
	Status:      "open",
	Priority:    1,
	IssueType:   "task",
	CreatedAt:   "2024-01-15T10:00:00Z",
	CreatedBy:   "user",
}

// SampleBead2 is a test bead with priority 2.
var SampleBead2 = Bead{
	ID:          "bd-002",
	Title:       "Test bead 2",
	Description: "Second test bead",
	Status:      "open",
	Priority:    2,
	IssueType:   "task",
	CreatedAt:   "2024-01-15T11:00:00Z",
	CreatedBy:   "user",
}

// SampleStateJSON is a sample .atari/state.json file content.
var SampleStateJSON = `{
  "running": true,
  "started_at": "2024-01-15T10:00:00Z",
  "current_bead": "bd-001",
  "beads_completed": 5,
  "total_cost_usd": 1.25,
  "history": {
    "bd-001": {"id": "bd-001", "status": "working", "attempts": 1, "last_attempt": "2024-01-15T10:30:00Z"}
  }
}`

// EmptyStateJSON is an empty state file.
var EmptyStateJSON = `{
  "running": false,
  "started_at": "0001-01-01T00:00:00Z",
  "current_bead": "",
  "beads_completed": 0,
  "total_cost_usd": 0,
  "history": {}
}`
