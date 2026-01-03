# Quick Bead Creation

Create a single bd issue from the current conversation context. This is a lightweight alternative to bd-plan for when you need to quickly capture one piece of work.

## Context Sources

This command receives context from two sources:
1. **Conversation history** - All messages above inform requirements, decisions, and scope
2. **Arguments** - Additional instructions passed when invoking the command (see $ARGUMENTS at end)

## Tools Used

- **Task (Explore subagent)** - Quick codebase exploration for relevant files and verification commands
- **AskUserQuestion** - Clarify ambiguities with dropdown options
- **bd CLI** - Issue creation and management

---

## Step 1: Extract from Conversation

Review the conversation history above to identify:
- **What**: The specific task or bug to address
- **Why**: The motivation or problem being solved
- **Scope**: What's included and excluded
- **Acceptance criteria**: How to know when it's done

If the user provided additional instructions below, incorporate those as well.

## Step 2: Gather Context (Quick)

### Relevant Files
If the conversation doesn't already make the relevant files clear, run a **quick** Explore query (model: "haiku") to identify:
- Files that will need modification
- Related test files
- Configuration files if applicable

Do NOT run Explore if the conversation already establishes the relevant files clearly.

### Verification Commands
Run a focused Explore query to find exact development commands:
```
Find the ACTUAL commands used in this project for verification. Search in order:
1. mise.toml / .mise.toml (mise task runner)
2. package.json scripts / pyproject.toml / Makefile / Justfile
3. .github/workflows (CI jobs are authoritative)

Report the EXACT command string for:
- Linting: e.g., `mise run lint`, `golangci-lint run`, `ruff check .`
- Tests: e.g., `mise run test`, `go test ./...`, `pytest`
- Type checking (if applicable): e.g., `mise run typecheck`, `mypy .`

Output format: "CATEGORY: [exact command]"
```

## Step 3: Clarify Ambiguities

Before creating the bead, use AskUserQuestion to clarify anything uncertain. Infer as much as possible from conversation context, but ask about:

**Always infer from context, only ask if unclear:**
- Priority (default: P2/normal) - infer from urgency words like "critical", "blocking", "minor", "nice to have"
- Scope boundaries - if the task could reasonably be interpreted multiple ways

**Only ask if mentioned in conversation:**
- Dependencies - if other beads were discussed as related/blocking
- Implementation approach - if multiple valid approaches were discussed

**Always ask:**
- Epic linkage - "Should this be linked to an existing epic?" (provide list if epics exist)

Use quick dropdown options rather than open-ended questions. Example:
```
questions:
  - question: "What priority should this bead have?"
    header: "Priority"
    options:
      - label: "P2 - Normal (Recommended)"
        description: "Standard priority for most tasks"
      - label: "P1 - High"
        description: "Important, should be done soon"
      - label: "P0 - Critical"
        description: "Blocking issue, needs immediate attention"
      - label: "P3 - Low"
        description: "Nice to have, can wait"
```

## Step 4: Create the Bead

Create the bd issue using the bd-issue-tracking skill. The issue must include:

1. **Clear title** (50 chars max, imperative voice)
2. **Description** with:
   - What and why (1-4 sentences)
   - Relevant files discovered
   - Acceptance criteria
3. **Verification section** with discovered commands:
   ```
   ## Verification
   - [ ] `[discovered lint command]` passes
   - [ ] `[discovered test command]` passes
   - [ ] `[discovered typecheck command]` passes (if applicable)
   ```
4. **Note**: "If implementation reveals new issues, create separate bd issues for investigation"

```bash
bd create "Title here" --priority <inferred-priority> --description "$(cat <<'EOF'
# Description
[What and why]

# Relevant Files
- path/to/file.go - [brief reason]
- path/to/file_test.go - [brief reason]

# Acceptance Criteria
- [ ] [Specific criterion 1]
- [ ] [Specific criterion 2]

# Verification
- [ ] `[lint command]` passes
- [ ] `[test command]` passes

If implementation reveals new issues, create separate bd issues for investigation.
EOF
)" --json
```

## Step 5: Epic Linkage (if applicable)

If the user confirmed epic linkage, add the parent-child relationship:
```bash
bd dep add <new-bead-id> <epic-id> --type parent-child
```

## Step 6: Output Summary

After creating the bead, output a clear summary:

```
Created: <bead-id>
Title: <title>
Priority: P<n>
Status: open

Description summary:
<2-3 sentence summary of what the bead covers>

Linked to epic: <epic-id> (or "None")
```

If the bead description is short, show the full description instead of a summary.

---

## Additional Instructions

$ARGUMENTS
