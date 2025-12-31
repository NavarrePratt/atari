# Repository Checklist (for agents and humans)

Use this doc as a quick operational guide when editing or validating bd-drain.

## Validate changes

- Run `mise run test` to verify all Go tests pass.
- Run `mise run build` to verify the binary compiles.
- Run `mise run bake` to verify Docker image builds (includes tests).

## Patterns to follow

- Backend: Cobra + Viper config, structured logging with slog, graceful shutdown (`cmd/AGENTS.md`).
- State: JSON state file at `.bd-drain/state.json` for persistence across restarts.
- Events: Unified event stream from Claude output, bd activity, and internal state changes.

## Directory structure

```
cmd/bd-drain/       # CLI entrypoint (Cobra/Viper)
internal/           # Non-exported packages
  controller/       # Main orchestration loop
  workqueue/        # Work discovery and selection
  session/          # Claude process management
  events/           # Event routing and sinks
  bdactivity/       # BD activity stream watcher
  daemon/           # Daemon mode and RPC
  tui/              # Terminal UI (bubbletea)
  config/           # Configuration loading
  shutdown/         # Graceful shutdown helper
docs/               # Design and implementation docs
```

## Tooling pins

- See `.mise.toml` for Go version and task definitions.
- Prefer `mise run <task>` over raw commands.

## House rules

- Keep docs self-contained.
- Favor structured logging (slog) over fmt.Print.
- Persist state on significant events for crash recovery.
- Single Claude session at a time (no parallel execution).
