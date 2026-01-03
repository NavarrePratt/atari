# Issue Tracking with bd

Track work with `bd` for persistent context across sessions.

## Quick Commands

| Task | Command |
|------|---------|
| Find ready work | `bd ready --json` |
| Start work | `bd update bd-xxx --status in_progress --json` |
| Checkpoint | `bd update bd-xxx --notes "COMPLETED: ..." --json` |
| Complete work | `bd close bd-xxx --reason "..." --json` |
| View details | `bd show bd-xxx --json` |

## Create Issue

```bash
bd create --title "Title" --description "$(cat <<'EOF'
# Description
What and why.

# Relevant files
Files from discovery.
EOF
)" --json
```

## Notes Format

```
COMPLETED: What was done
KEY DECISION: Why this approach
IN PROGRESS: Current state
NEXT: Immediate next step
```

## Do NOT Close If

- Tests failing
- Implementation partial
- Unresolved errors
- Integration tests not updated

Instead: `bd update bd-xxx --notes "BLOCKED: ..." --json`
