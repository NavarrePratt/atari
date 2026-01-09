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
