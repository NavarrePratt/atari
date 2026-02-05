<atari-managed>
# BR Integration

Use the br CLI to track work across sessions.

See detailed rules in:
- @rules/issue-tracking.md - br CLI patterns and issue management
- @rules/session-protocol.md - Session procedures and quality gates

## Quick Reference

### Session Startup (User-Initiated Only)

Run these commands ONLY when the user explicitly requests session initialization (e.g., "start session", "check for work", "what's ready?"). Do NOT run automatically after context compaction - if you were working on something before compaction, continue that work.

```bash
pwd && br prime && br ready --json && git log --oneline -5 && git status
```

### Issue Workflow

```bash
br ready --json                           # Find work
br update bd-xxx --status in_progress     # Claim it
# ... do work ...
br close bd-xxx --reason "Completed..."   # Close with reason
```

### Git Commits

Use `/commit` slash command for all commits - creates atomic, well-formatted commits matching project style.

## Issue Tracking Summary

Track all work with `br`. Create issues for test failures and bugs. Record meticulous notes for history.

**Priority levels**: 0=critical, 1=high, 2=normal, 3=low, 4=backlog

**Creating issues**: Title 50 chars max, imperative voice. Verbose descriptions with relevant files and snippets.

**Closing issues**: Always provide `--reason` with what was done and how verified. Never close if tests fail or implementation is partial.

**Dependencies**: `br dep add A B --type blocks` means A must complete before B.

## Session Protocol Summary

**Startup (user-initiated only)**: `br prime` -> `br ready` -> review git state. Do NOT run after context compaction.

**Work**: One issue at a time. Commit after each. Verify end-to-end.

**Completion**: File remaining work as issues. Close completed issues. Do NOT push.

## CRITICAL: Bead Closure

**You MUST close or reset beads before ending your session.** Beads left in_progress get stuck forever.

- Work complete: `br close bd-xxx --reason "Completed: ..."`
- Work incomplete: `br update bd-xxx --status open --notes "Needs: ..."`

Never leave beads in_progress.

## Quality Gates

Before committing:
- Code compiles/lints without errors
- All tests pass
- No hardcoded secrets
- Changes are minimal and focused
</atari-managed>
