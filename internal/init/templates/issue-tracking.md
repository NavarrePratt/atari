# Issue Tracking with bd

Track work with `bd` for persistent context across sessions.

## Quick Commands

| Task | Command |
|------|---------|
| Find ready work | `bd ready --json` |
| Start work | `bd update bd-xxx --status in_progress --json` |
| Checkpoint | `bd update bd-xxx --notes "COMPLETED: ...\nNEXT: ..." --json` |
| Complete work | `bd close bd-xxx --reason "..." --json` |
| View details | `bd show bd-xxx --json` |
| Add dependency | `bd dep add bd-A bd-B --type blocks` |

## Create Issue

```bash
bd create --title "Title" --description "$(cat <<'EOF'
# Description
What and why (1-4 sentences).

# Relevant files
Files and snippets from discovery.
EOF
)" --json
```

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

Instead: `bd update bd-xxx --notes "BLOCKED: ..." --json`

## Integration Test Requirements

When closing issues that involve:
- **API changes**: Verify existing integration tests still pass with new patterns
- **New features**: Add integration tests if feature interacts with external services
- **Bug fixes**: Add regression test if bug was user-facing

Before closing: Run integration tests using project-specific command (check project CLAUDE.md)
