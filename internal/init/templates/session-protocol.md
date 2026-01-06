# Session Protocol

## Session Startup (User-Initiated Only)

Run this sequence ONLY when the user explicitly requests it (e.g., "start session", "check for work").

**After context compaction**: Do NOT treat compaction as a new session. If you were actively working on something, continue that work. The compaction summary will indicate what you were doing.

```bash
pwd                    # Confirm working directory
bd prime               # Recover context
bd ready --json        # Find available work
git log --oneline -5   # Review recent state
git status             # Verify clean state
```

## Incremental Progress

Work on ONE issue at a time:

1. Select highest-priority issue from `bd ready`
2. Implement ONLY that feature
3. Commit with `/commit` slash command
4. Close: `bd close <id> --reason "Completed..." --json`
5. Verify feature works end-to-end
6. Move to next issue

Never batch multiple features into single commits.

## CRITICAL: Bead Closure Requirement

**You MUST close beads before ending your session.** Failure to close beads causes them to get stuck in_progress forever, requiring manual intervention.

Before ending your session:
1. Run `bd show <id> --json` to verify the bead status
2. If work is complete: `bd close <id> --reason "Completed: <what was done>"`
3. If work is NOT complete: `bd update <id> --status open --notes "Needs: <what remains>"`

Never leave a bead in_progress - either close it or reset it to open.

## Session Completion

```bash
bd create "Follow-up: ..." --json    # File remaining work
bd close <id> --reason "..." --json  # Close completed issues
# Run quality validation (lint, test, type-check)
# Use /commit for atomic commits
git push                              # MUST succeed
```

Session is NOT complete until `git push` succeeds.

## Clean State Requirement

Each session must leave code production-ready:
- No major bugs introduced
- Well-documented changes
- Mergeable to main without cleanup
- All tests passing

# Quality Gates

Before committing:
- Code compiles/lints without errors
- All existing tests pass
- New code has appropriate test coverage
- No hardcoded secrets or credentials
- Changes are minimal and focused

# Integration Test Verification

After implementing changes that affect core functionality:

1. **Check if integration tests need updates**: If you modified API behavior, added features,
   or changed how existing functionality works, verify integration tests still pass
2. **Update integration tests**: When tests use outdated patterns, update them to match
   current API patterns
3. **Run integration tests**: Execute the project's integration test command before closing
   issues (check project CLAUDE.md for specific commands)
4. **Add new integration tests**: When implementing new features that interact with external
   services, add corresponding integration test coverage

**Warning**: Do not get stuck in planning loops when tests fail. If integration tests fail
after code changes, diagnose and fix them rather than reverting to planning mode repeatedly.
Check test patterns match current API usage.

# Code Style

- Read before modifying: understand existing code first
- Match existing style: indentation, naming, patterns
- Minimal changes: only modify what's necessary
- Delete unused code: remove completely, don't comment out
- No over-engineering - three similar lines > premature abstraction

# Git Commits

Use `/commit` slash command (git-commit skill) for all commits:
- Analyzes git history to match project's commit style
- Creates atomic, logically grouped commits
- Follows best practices: imperative mood, 50-char subject, explanatory body
- Supports interactive staging for fine-grained control

# Communication Standards

- Be explicit - state instructions directly
- Provide context - explain why, not just what
- Use positive framing - say what TO do, not what to avoid
- Be concise - no filler words
- Match specificity to complexity
