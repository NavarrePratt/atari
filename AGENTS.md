# Repository Checklist (for agents and humans)

Use this doc as a quick operational guide when editing or validating atari.

## Validate changes

- Run `mise run test` to verify all Go tests pass.
- Run `mise run build` to verify the binary compiles.
- Run `mise run bake` to verify Docker image builds (includes tests).

## Patterns to follow

- Backend: Cobra + Viper config, structured logging with slog, graceful shutdown (`cmd/AGENTS.md`).
- State: JSON state file at `.atari/state.json` for persistence across restarts.
- Events: Unified event stream from Claude output, bd activity, and internal state changes.

## Tooling pins

- See `.mise.toml` for Go version and task definitions.
- Prefer `mise run <task>` over raw commands.

## House rules

- Keep docs self-contained.
- Favor structured logging (slog) over fmt.Print.
- Persist state on significant events for crash recovery.
- Single Claude session at a time (no parallel execution).

## Landing the Plane (Session Completion)

**When ending a work session**, complete the steps below.

**WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Commit all changes** - Use `/commit` for atomic commits
5. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Do NOT push to remote - commits should remain local for user review
- All work must be committed before ending session
- Close or reset all beads (never leave in_progress)
