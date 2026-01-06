package testutil

// Graph visualization fixtures for testing the TUI graph components.

// GraphActiveBeadsJSON is a sample bd list --json response for active beads
// with dependencies and parent-child relationships.
var GraphActiveBeadsJSON = `[
  {
    "id": "bd-epic-001",
    "title": "Epic: User Authentication",
    "description": "Implement user authentication system",
    "status": "open",
    "priority": 1,
    "issue_type": "epic",
    "created_at": "2024-01-15T10:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T10:00:00Z",
    "dependency_count": 0,
    "dependent_count": 2
  },
  {
    "id": "bd-task-001",
    "title": "Implement login form",
    "description": "Create login form component",
    "status": "in_progress",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T11:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T12:00:00Z",
    "dependency_count": 1,
    "dependent_count": 1,
    "dependencies": [
      {
        "id": "bd-epic-001",
        "title": "Epic: User Authentication",
        "status": "open",
        "dependency_type": "parent-child"
      }
    ]
  },
  {
    "id": "bd-task-002",
    "title": "Add session management",
    "description": "Implement session storage and validation",
    "status": "blocked",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T11:30:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T12:30:00Z",
    "dependency_count": 2,
    "dependent_count": 0,
    "dependencies": [
      {
        "id": "bd-epic-001",
        "title": "Epic: User Authentication",
        "status": "open",
        "dependency_type": "parent-child"
      },
      {
        "id": "bd-task-001",
        "title": "Implement login form",
        "status": "in_progress",
        "dependency_type": "blocks"
      }
    ]
  }
]`

// GraphBacklogBeadsJSON is a sample bd list --json response for backlog beads.
var GraphBacklogBeadsJSON = `[
  {
    "id": "bd-deferred-001",
    "title": "Nice-to-have feature",
    "description": "A feature for later",
    "status": "deferred",
    "priority": 4,
    "issue_type": "task",
    "created_at": "2024-01-10T10:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-10T10:00:00Z",
    "dependency_count": 0,
    "dependent_count": 0
  }
]`

// GraphEmptyBeadsJSON is an empty bd list response.
var GraphEmptyBeadsJSON = `[]`

// GraphSingleBeadJSON is a bd list response with a single bead and no dependencies.
var GraphSingleBeadJSON = `[
  {
    "id": "bd-single-001",
    "title": "Standalone task",
    "description": "A task with no dependencies",
    "status": "open",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T10:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T10:00:00Z",
    "dependency_count": 0,
    "dependent_count": 0
  }
]`

// GraphMixedWithAgentJSON is a bd list response that includes agent beads
// which should be filtered out from graph visualization.
var GraphMixedWithAgentJSON = `[
  {
    "id": "bd-task-001",
    "title": "Regular task",
    "description": "A normal task",
    "status": "open",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T10:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T10:00:00Z"
  },
  {
    "id": "bd-agent-001",
    "title": "Agent tracking bead",
    "description": "Internal agent state",
    "status": "open",
    "priority": 2,
    "issue_type": "agent",
    "created_at": "2024-01-15T10:00:00Z",
    "created_by": "atari",
    "updated_at": "2024-01-15T10:00:00Z"
  },
  {
    "id": "bd-task-002",
    "title": "Another task",
    "description": "Another normal task",
    "status": "in_progress",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T11:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T11:00:00Z"
  }
]`

// JSONL fixtures for testing the JSONLReader.
// These represent the format stored in .beads/issues.jsonl files.

// GraphJSONLBasic is a minimal JSONL sample with basic beads.
var GraphJSONLBasic = `{"id":"bd-001","title":"First task","description":"A basic task","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-002","title":"Second task","description":"Another task","status":"in_progress","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}
{"id":"bd-003","title":"Third task","description":"A blocked task","status":"blocked","priority":2,"issue_type":"task","created_at":"2024-01-15T12:00:00Z","created_by":"user","updated_at":"2024-01-15T12:00:00Z"}`

// GraphJSONLWithDependencies is a JSONL sample with dependency relationships.
var GraphJSONLWithDependencies = `{"id":"bd-epic-001","title":"Epic: Auth System","description":"Authentication epic","status":"open","priority":1,"issue_type":"epic","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-task-001","title":"Login form","description":"Create login form","status":"in_progress","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z","dependencies":[{"issue_id":"bd-task-001","depends_on_id":"bd-epic-001","type":"parent-child"}]}
{"id":"bd-task-002","title":"Session mgmt","description":"Session management","status":"blocked","priority":2,"issue_type":"task","created_at":"2024-01-15T12:00:00Z","created_by":"user","updated_at":"2024-01-15T12:00:00Z","dependencies":[{"issue_id":"bd-task-002","depends_on_id":"bd-epic-001","type":"parent-child"},{"issue_id":"bd-task-002","depends_on_id":"bd-task-001","type":"blocks"}]}`

// GraphJSONLWithDeleted is a JSONL sample with deleted entries that should be filtered.
var GraphJSONLWithDeleted = `{"id":"bd-001","title":"Active task","description":"Should be visible","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-002","title":"Deleted task","description":"Should be filtered","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z","deleted_at":"2024-01-16T10:00:00Z"}
{"id":"bd-003","title":"Another active","description":"Should be visible","status":"in_progress","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}`

// GraphJSONLWithAgent is a JSONL sample with agent beads that should NOT be filtered by ReadAll.
// Agent filtering happens at a higher level (FetchActive, etc.), not in ReadAll.
var GraphJSONLWithAgent = `{"id":"bd-task-001","title":"Regular task","description":"A normal task","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-agent-001","title":"Agent bead","description":"Internal agent state","status":"open","priority":2,"issue_type":"agent","created_at":"2024-01-15T10:00:00Z","created_by":"atari","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-task-002","title":"Another task","description":"Another normal task","status":"in_progress","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}`

// GraphJSONLMalformed is a JSONL sample with malformed lines.
var GraphJSONLMalformed = `{"id":"bd-001","title":"Valid task","description":"First valid","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{this is not valid json}
{"id":"bd-003","title":"Another valid","description":"Third valid","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T12:00:00Z","created_by":"user","updated_at":"2024-01-15T12:00:00Z"}`

// GraphJSONLWithBlankLines is a JSONL sample with blank lines that should be skipped.
var GraphJSONLWithBlankLines = `{"id":"bd-001","title":"First task","description":"Task one","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}

{"id":"bd-002","title":"Second task","description":"Task two","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T11:00:00Z","created_by":"user","updated_at":"2024-01-15T11:00:00Z"}

{"id":"bd-003","title":"Third task","description":"Task three","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T12:00:00Z","created_by":"user","updated_at":"2024-01-15T12:00:00Z"}`

// GraphJSONLMissingDeps is a JSONL sample where a dependency references a non-existent bead.
var GraphJSONLMissingDeps = `{"id":"bd-001","title":"Task with missing dep","description":"Has dep on non-existent bead","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z","dependencies":[{"issue_id":"bd-001","depends_on_id":"bd-nonexistent","type":"blocks"}]}`

// GraphJSONLMixedStatus is a JSONL sample with beads in various statuses for testing filtered reads.
var GraphJSONLMixedStatus = `{"id":"bd-open","title":"Open task","description":"Status open","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-progress","title":"In progress task","description":"Status in_progress","status":"in_progress","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-blocked","title":"Blocked task","description":"Status blocked","status":"blocked","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-deferred","title":"Deferred task","description":"Status deferred","status":"deferred","priority":4,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}
{"id":"bd-closed-recent","title":"Recently closed","description":"Closed within 7 days","status":"closed","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z","closed_at":"2024-01-20T10:00:00Z"}
{"id":"bd-agent","title":"Agent bead","description":"Should be filtered","status":"open","priority":2,"issue_type":"agent","created_at":"2024-01-15T10:00:00Z","created_by":"atari","updated_at":"2024-01-15T10:00:00Z"}`

// GraphJSONLClosedBeads is a JSONL sample with closed beads for testing ReadClosed filtering.
// Uses RFC3339Nano format to test fractional second parsing.
var GraphJSONLClosedBeads = `{"id":"bd-closed-1","title":"Closed yesterday","description":"Should appear in 7-day filter","status":"closed","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z","closed_at":"2024-01-19T10:00:00.123456789Z"}
{"id":"bd-closed-2","title":"Closed last month","description":"Should NOT appear in 7-day filter","status":"closed","priority":2,"issue_type":"task","created_at":"2024-01-01T10:00:00Z","created_by":"user","updated_at":"2024-01-01T10:00:00Z","closed_at":"2024-01-01T10:00:00Z"}
{"id":"bd-open-1","title":"Still open","description":"Not closed","status":"open","priority":2,"issue_type":"task","created_at":"2024-01-15T10:00:00Z","created_by":"user","updated_at":"2024-01-15T10:00:00Z"}`

// GraphComplexHierarchyJSON is a bd list response with a more complex hierarchy.
var GraphComplexHierarchyJSON = `[
  {
    "id": "bd-root",
    "title": "Root Epic",
    "description": "Top-level epic",
    "status": "open",
    "priority": 1,
    "issue_type": "epic",
    "created_at": "2024-01-15T10:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T10:00:00Z"
  },
  {
    "id": "bd-child-a",
    "title": "Child A",
    "description": "First child task",
    "status": "open",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T11:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T11:00:00Z",
    "dependencies": [
      {"id": "bd-root", "title": "Root Epic", "status": "open", "dependency_type": "parent-child"}
    ]
  },
  {
    "id": "bd-child-b",
    "title": "Child B",
    "description": "Second child task",
    "status": "open",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T11:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T11:00:00Z",
    "dependencies": [
      {"id": "bd-root", "title": "Root Epic", "status": "open", "dependency_type": "parent-child"}
    ]
  },
  {
    "id": "bd-grandchild",
    "title": "Grandchild",
    "description": "Depends on both children",
    "status": "blocked",
    "priority": 2,
    "issue_type": "task",
    "created_at": "2024-01-15T12:00:00Z",
    "created_by": "user",
    "updated_at": "2024-01-15T12:00:00Z",
    "dependencies": [
      {"id": "bd-child-a", "title": "Child A", "status": "open", "dependency_type": "blocks"},
      {"id": "bd-child-b", "title": "Child B", "status": "open", "dependency_type": "blocks"}
    ]
  }
]`
