# Issue Tracking Skill

Track and manage work with issue tracker for persistent context across sessions and compaction events. Use for work needing dependencies, recovery after compaction, or multi-session tracking.

## Trigger

Use when:
- Starting work on an issue
- Updating issue status or notes
- Creating new issues for discovered work
- Closing completed work
- Managing dependencies between issues

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

# Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

# Verification
- [ ] `[lint command]` passes
- [ ] `[test command]` passes
EOF
)" --json
```

## Notes Format

```
COMPLETED: What was done
KEY DECISION: Why this approach
IN PROGRESS: Current state
NEXT: Immediate next step
BLOCKED: What is blocking (if applicable)
```

## Priority Levels

0=critical, 1=high, 2=normal, 3=low, 4=backlog

## Close Criteria

Do NOT close if:
- Tests failing
- Implementation partial
- Unresolved errors
- Integration tests not updated

Instead: `br update bd-xxx --notes "BLOCKED: ..." --json`

## Session Recovery

After context compaction:
1. Run `br prime` to recover context
2. Check `br ready --json` for current work
3. Resume in_progress issue from notes
