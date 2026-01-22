# Issue Tracking with br

Track work with `br` for persistent context across sessions.

## Quick Commands

| Task | Command |
|------|---------|
| Find ready work | `br ready --json` |
| Start work | `br update bd-xxx --status in_progress --json` |
| Checkpoint | `br update bd-xxx --notes "COMPLETED: ...\nNEXT: ..." --json` |
| Complete work | `br close bd-xxx --reason "..." --json` |
| View details | `br show bd-xxx --json` |
| Add dependency | `br dep add bd-A bd-B --type blocks` |

## Create Issue

```bash
br create --title "Title" --description "$(cat <<'EOF'
# Description
What and why (1-4 sentences).

# Relevant files
Files and snippets from discovery.
EOF
)" --json
```

### Deferred Status for Batch Creation

When creating multiple beads under an epic, use `--status deferred` to prevent them from being picked up before dependencies are set:

```bash
# Create beads in deferred status
br create "First task" --status deferred --description "..." --json
br create "Second task" --status deferred --description "..." --json

# Set up dependencies
br dep add bd-001 bd-002 --type blocks

# Publish all beads when ready
br update bd-001 --status open
br update bd-002 --status open
```

This ensures atari will not pick up any beads until the entire plan is ready and properly sequenced.

**CRITICAL: No ANSI escape codes in issues.** Never copy colored terminal output into titles or descriptions. Write text from scratch - paraphrase rather than copy. ANSI codes (`\x1b[`, `^[[`) stored in issues propagate to commit messages as garbage characters.

## Notes Format

```
COMPLETED: What was done
KEY DECISION: Why this approach
IN PROGRESS: Current state
NEXT: Immediate next step
```

## Checkpoint Triggers

- Token usage > 70%
- Major milestone reached
- Hit a blocker
- Before asking user for input

## Priority Levels

0=critical, 1=high, 2=normal, 3=low, 4=backlog

## Do NOT Close If

- Tests failing
- Implementation partial
- Unresolved errors
- Integration tests not updated for API changes
- New features lack integration test coverage (when applicable)

Instead: `br update bd-xxx --notes "BLOCKED: ..." --json`

## Integration Test Requirements

When closing issues that involve:
- **API changes**: Verify existing integration tests still pass with new patterns
- **New features**: Add integration tests if feature interacts with external services
- **Bug fixes**: Add regression test if bug was user-facing

Before closing: Run integration tests using project-specific command (check project CLAUDE.md)
