# Design Document

High-level architecture overview for Atari.

For detailed component documentation, see the [components/](components/) directory.

## Goals

1. **Autonomous execution**: Run Claude Code sessions to completion without human intervention
2. **Observability**: Unified real-time stream of Claude and bd events
3. **Resilience**: Survive crashes/interrupts and resume from last known state
4. **Controllability**: Pause, resume, stop via CLI commands
5. **Simplicity**: Single daemon, single Claude worker, clear state model

## Non-Goals

1. Parallel execution (multiple simultaneous Claude sessions)
2. Distributed operation (multiple machines)
3. Web UI (terminal TUI only)
4. Hard cost limits (monitoring only)
5. Session chaining across beads (each bead gets fresh session)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          atari daemon                           │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      Controller                            │  │
│  │            State: idle | working | paused | stopping       │  │
│  └─────────────────────┬─────────────────────────────────────┘  │
│           ┌────────────┼────────────┬────────────┐              │
│           ▼            ▼            ▼            ▼              │
│    ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐         │
│    │  Work    │ │ Session  │ │  Event   │ │   BD     │         │
│    │  Queue   │ │ Manager  │ │  Router  │ │ Activity │         │
│    └──────────┘ └──────────┘ └──────────┘ └──────────┘         │
│           │            │            │            │              │
│           └────────────┴─────┬──────┴────────────┘              │
│                              ▼                                  │
│    ┌─────────────────────────────────────────────────────────┐  │
│    │                     Event Bus                            │  │
│    └─────────────────────┬───────────────────────────────────┘  │
│           ┌──────────────┼──────────────┬───────────┐           │
│           ▼              ▼              ▼           ▼           │
│    ┌──────────┐   ┌──────────┐   ┌──────────┐ ┌──────────┐     │
│    │   Log    │   │  State   │   │   TUI    │ │  Notify  │     │
│    │   Sink   │   │   Sink   │   │   Sink   │ │   Sink   │     │
│    └──────────┘   └──────────┘   └──────────┘ └──────────┘     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      External Processes                         │
│  ┌──────────┐      ┌──────────────┐      ┌──────────┐          │
│  │  claude  │      │ bd activity  │      │    bd    │          │
│  │   -p     │      │   --follow   │      │  daemon  │          │
│  └──────────┘      └──────────────┘      └──────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Components

| Component | Purpose | Documentation |
|-----------|---------|---------------|
| Controller | Main orchestration and state machine | [components/controller.md](components/controller.md) |
| Work Queue | Polls bd ready, selects beads, tracks history | [components/workqueue.md](components/workqueue.md) |
| Session Manager | Spawns Claude, parses stream output | [components/session.md](components/session.md) |
| Event Router | Merges events from all sources | [components/events.md](components/events.md) |
| BD Activity | Watches bd activity stream | [components/bdactivity.md](components/bdactivity.md) |
| Log Sink | Persists events to JSON file | [components/sinks.md](components/sinks.md) |
| State Sink | Maintains runtime state | [components/sinks.md](components/sinks.md) |
| TUI Sink | Terminal UI display | [components/tui.md](components/tui.md) |
| Observer | Interactive Q&A about events (future) | [components/observer.md](components/observer.md) |
| Notifications | Webhooks for external alerts | [components/notifications.md](components/notifications.md) |
| Daemon | Background mode and RPC | [components/daemon.md](components/daemon.md) |

## State Machine

```
     init
       │
       ▼
    ┌──────┐    bead available    ┌─────────┐
    │ idle │ ──────────────────▶  │ working │
    └──────┘                      └─────────┘
       ▲                               │
       │   no more beads               │ pause
       └───────────────────────────────┤
                                       ▼
                                 ┌─────────┐
                                 │ paused  │
                                 └─────────┘
                                       │ stop
                                       ▼
                                 ┌─────────┐
                                 │ stopped │
                                 └─────────┘
```

See [components/controller.md](components/controller.md) for full state machine details.

## Data Flow

### Event Flow

1. **Claude session** emits stream-json events → Session Manager parses → Event Router
2. **BD activity** emits mutation events → BD Activity parses → Event Router
3. **Controller** emits internal events (iteration start/end) → Event Router
4. **Event Router** broadcasts to all sinks (log, state, TUI, notifications)

### State Persistence

- State saved to `.atari/state.json` on every significant event
- Includes: current bead, stats, bead history
- Enables resume after crash

## External Integrations

| System | How Atari Integrates |
|--------|---------------------|
| Claude Code | `claude -p --output-format stream-json` (uses global Claude config) |
| bd ready | Poll `bd ready --json` for available work |
| bd activity | `bd activity --follow --json` for real-time events |
| bd agent | Track controller state via `bd agent state atari <state>` |

Note: Claude model, max-turns, and other settings come from the user's global Claude config (`~/.claude/settings.json`). Atari only passes minimal required flags.

### Beads Integration Philosophy

Atari integrates with beads through the **CLI interface** only. We do NOT import beads Go packages directly because:

1. **Stability**: CLI is the public, stable API - internal packages may change
2. **Decoupling**: No version coupling between atari and beads
3. **Simplicity**: Easy to test with mock commands

See [BEADS_INTEGRATION.md](BEADS_INTEGRATION.md) for detailed integration patterns.

## CLI Interface

| Command | Purpose |
|---------|---------|
| `atari start` | Start the drain controller |
| `atari stop` | Stop the daemon |
| `atari pause` | Pause after current bead |
| `atari resume` | Resume from pause |
| `atari status` | Show current state |
| `atari events` | View event stream |
| `atari init` | Initialize Claude configuration |

See [cli/commands.md](cli/commands.md) for full CLI documentation.

## Configuration

Configuration via YAML file, environment variables, and CLI flags.

Key settings:
- `claude.model`: Model to use (opus, sonnet)
- `claude.max_turns`: Maximum turns per session
- `workqueue.label`: Filter beads by label
- `notifications.*`: Webhook configuration

See [config/configuration.md](config/configuration.md) for full configuration reference.

## Error Handling

| Scenario | Action |
|----------|--------|
| Claude session fails | Record failure, apply backoff, continue |
| bd ready fails | Retry with backoff |
| Session timeout | Kill process, reset bead |
| State file corrupt | Start fresh with warning |

## File Layout

```
.atari/
├── config.yaml     # Configuration
├── state.json      # Persistent state
├── atari.log       # Event log
├── atari.sock      # Unix socket (daemon mode)
└── atari.pid       # PID file (daemon mode)
```

## Future Considerations

Documented but explicitly out of scope for initial implementation:

1. Parallel execution (multiple Claude workers)
2. Session resume across beads
3. Web dashboard
4. Webhooks (inbound)
5. Hard cost limits
6. Priority boosting
7. GitHub Actions integration
