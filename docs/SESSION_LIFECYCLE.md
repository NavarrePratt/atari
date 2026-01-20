# Session and Turn Lifecycle

This document describes how Atari manages Claude Code sessions in relation to bead tasks, including turn progression, control actions (pause/resume/stop), and error handling.

## Overview

Atari follows a simple model:
- **One bead = one session**: Each bead gets a dedicated Claude Code session
- **One session = multiple turns**: Claude completes work through tool-use cycles called turns
- **Sequential execution**: Only one session runs at a time

## Key Concepts

| Term | Definition |
|------|------------|
| **Bead** | A task/issue from the bd system (`bd ready --json`) |
| **Session** | A single Claude Code process execution (`claude -p --output-format stream-json`) |
| **Turn** | A tool-use cycle: Claude calls tools, receives results, continues |
| **Iteration** | Controller's processing of one bead (includes session + verification) |

## High-Level Architecture

```
                            ATARI DAEMON
 ┌────────────────────────────────────────────────────────────────┐
 │                                                                │
 │  ┌──────────────────────────────────────────────────────────┐  │
 │  │                     CONTROLLER                           │  │
 │  │        State: idle | working | paused | stopping         │  │
 │  └─────────────────────────┬────────────────────────────────┘  │
 │                            │                                   │
 │          ┌─────────────────┼─────────────────┐                 │
 │          ▼                 ▼                 ▼                 │
 │   ┌────────────┐   ┌─────────────┐   ┌────────────┐            │
 │   │  WORKQUEUE │   │   SESSION   │   │   EVENT    │            │
 │   │   Manager  │   │   Manager   │   │   Router   │            │
 │   └──────┬─────┘   └──────┬──────┘   └────────────┘            │
 │          │                │                                    │
 │          │                ▼                                    │
 │          │         ┌─────────────┐                             │
 │          │         │   PARSER    │                             │
 │          │         │  (turns)    │                             │
 │          │         └─────────────┘                             │
 └──────────┼───────────────────────────────────────────────────-─┘
            │
            ▼
     ┌─────────────┐              ┌─────────────┐
     │  bd ready   │              │   claude    │
     │   --json    │              │     -p      │
     └─────────────┘              └─────────────┘
```

## Session Lifecycle

### Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          FULL SESSION LIFECYCLE                             │
└─────────────────────────────────────────────────────────────────────────────┘

     WORKQUEUE                    CONTROLLER                    SESSION
         │                            │                            │
         │    ◀── poll ───            │                            │
         │    bd ready --json         │                            │
         │                            │                            │
         ├──── bead found ───────────▶│                            │
         │    {id, title, priority}   │                            │
         │                            │                            │
         │                            ├──── state: WORKING ────────│
         │                            │                            │
         │                            │    ┌──── spawn ───────────▶│
         │                            │    │    claude -p          │
         │                            │    │    --output-format    │
         │                            │    │    stream-json        │
         │                            │    │                       │
         │                            │    │                ┌──────┴────────┐
         │                            │    │                │  TURN 1       │
         │                            │    │                │  ┌─────────┐  │
         │                            │    │                │  │tool_use │  │
         │                            │    │                │  │  Bash   │─ ┼──▶ executes
         │                            │    │                │  └─────────┘  │
         │                            │    │                │  ┌─────────┐  │
         │                            │    │                │  │tool_    │  │
         │                            │    │                │  │result   │◀─┼── result
         │                            │    │                │  └─────────┘  │
         │                            │    │                └───────┬───────┘
         │                            │    │                        │
         │                            │    │  ◀─ TurnCompleteEvent ─┤
         │                            │    │                        │
         │                            │    │                ┌───────┴───────┐
         │                            │    │                │  TURN 2       │
         │                            │    │                │  ┌─────────┐  │
         │                            │    │                │  │tool_use │  │
         │                            │    │                │  │  Read   │──┼──▶ reads file
         │                            │    │                │  └─────────┘  │
         │                            │    │                │  ┌─────────┐  │
         │                            │    │                │  │tool_use │  │
         │                            │    │                │  │  Edit   │──┼──▶ edits file
         │                            │    │                │  └─────────┘  │
         │                            │    │                │  ┌─────────┐  │
         │                            │    │                │  │tool_    │  │
         │                            │    │                │  │results  │◀─┼── results
         │                            │    │                │  └─────────┘  │
         │                            │    │                └───────┬───────┘
         │                            │    │                        │
         │                            │    │  ◀─ TurnCompleteEvent ─┤
         │                            │    │                        │
         │                            │    │                    ... │
         │                            │    │                        │
         │                            │    │                ┌───────┴───────┐
         │                            │    │                │  TURN N       │
         │                            │    │                │  bd close     │
         │                            │    │                │  (bead done)  │
         │                            │    │                └───────┬───────┘
         │                            │    │                        │
         │                            │    │  ◀── SessionEndEvent ──┤
         │                            │    │      {num_turns,       │
         │                            │    │       cost, duration}  │
         │                            │    │                        │
         │                            │◀───┘                        │
         │                            │                             │
         │                            ├──── verify bead closed ─────│
         │                            │     bd show <id> --json     │
         │                            │                             │
         │      ◀── record success ───┤                             │
         │                            │                             │
         │                            ├──── state: IDLE ────────────│
         │                            │                             │
         ▼                            ▼                             ▼
```

### Turn Definition

A **turn** is defined as a complete tool-use cycle:

```
┌─────────────────────────────────────────────────────────────┐
│                         ONE TURN                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Claude emits ASSISTANT message with tool_use blocks     │
│                                                             │
│     {"type": "assistant", "message": {"content": [          │
│       {"type": "tool_use", "name": "Bash", ...},            │
│       {"type": "tool_use", "name": "Read", ...}             │
│     ]}}                                                     │
│                                                             │
│  2. Tools execute (may run in parallel)                     │
│                                                             │
│  3. USER message with matching tool_result blocks           │
│                                                             │
│     {"type": "user", "message": {"content": [               │
│       {"type": "tool_result", "tool_use_id": "t1", ...},    │
│       {"type": "tool_result", "tool_use_id": "t2", ...}     │
│     ]}}                                                     │
│                                                             │
│  4. Turn boundary reached when:                             │
│     pending_tool_uses == 0 (all tools have results)         │
│                                                             │
│  5. TurnCompleteEvent emitted                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Session Completion Outcomes

```
                              SESSION ENDS
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
              ┌──────────┐  ┌──────────┐  ┌──────────┐
              │  NORMAL  │  │  ERROR   │  │ TIMEOUT  │
              │   EXIT   │  │   EXIT   │  │  (idle)  │
              └────┬─────┘  └────┬─────┘  └────┬─────┘
                   │             │             │
                   ▼             ▼             ▼
              ┌──────────────────────────────────────┐
              │         CHECK BEAD STATUS            │
              │         bd show <id> --json          │
              └────────────────┬─────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
        ┌──────────┐    ┌──────────┐    ┌───────────┐
        │  CLOSED  │    │   OPEN   │    │IN_PROGRESS│
        │ (success)│    │ (retry)  │    │  (stuck)  │
        └────┬─────┘    └────┬─────┘    └────┬──────┘
             │               │               │
             ▼               ▼               ▼
        Record success  Record failure  Run follow-up
                                        session
```

## Controller State Machine

### States

| State | Description |
|-------|-------------|
| `idle` | Polling workqueue for ready beads |
| `working` | Claude session active on a bead |
| `paused` | Waiting for resume/stop signal |
| `stopping` | Graceful shutdown in progress |
| `stopped` | Daemon has stopped |

### State Transition Diagram

```
                              ┌──────────────┐
                              │     init     │
                              └──────┬───────┘
                                     │ start
                                     ▼
                    ┌────────────────────────────────┐
                    │                                │
          ┌────────▶│             IDLE               │◀────────┐
          │         │       (polling for beads)      │         │
          │         └───────────────┬────────────────┘         │
          │                         │                          │
          │                         │ bead available           │
          │                         ▼                          │
          │         ┌────────────────────────────────┐         │
          │         │                                │         │
          │         │            WORKING             │─────────┘
          │         │       (claude session)         │  session complete
          │         │                                │  (no pending signals)
          │         └───────────────┬────────────────┘
          │                         │
          │         ┌───────────────┼───────────────┐
          │         │ pause         │ stop          │ graceful pause
          │         │               │               │ (at turn boundary)
          │         ▼               ▼               ▼
          │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
          │  │             │ │             │ │  (working   │
          │  │   PAUSED    │ │  STOPPING   │ │   + pause   │
          │  │             │ │             │ │   pending)  │
          │  └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
          │         │               │               │
          │  resume │               │ wait for      │ session ends
          └─────────┘               │ session       └───────┐
                                    ▼                       ▼
                              ┌─────────────┐        ┌─────────────┐
                              │             │        │             │
                              │   STOPPED   │◀───────│   PAUSED    │
                              │             │  stop  │             │
                              └─────────────┘        └─────────────┘
```

## Control Actions

### Pause

Pause waits for the current iteration (bead processing) to complete before stopping.

```
                          PAUSE FLOW
     ┌─────────────────────────────────────────────────┐
     │                                                 │
     │  User: atari pause                              │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == IDLE:                       │    │
     │  │   -> immediately transition to PAUSED   │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == WORKING:                    │    │
     │  │   -> set pauseSignal                    │    │
     │  │   -> session continues to completion    │    │
     │  │   -> after session ends: PAUSED         │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     └─────────────────────────────────────────────────┘

     Timeline (working state):
     ┌──────────────────────────────────────────────────────────────┐
     │                                                              │
     │  pause          session continues           session ends     │
     │  requested      normally                    -> PAUSED        │
     │     │                │                           │           │
     │     ▼                ▼                           ▼           │
     │ ────●────────────────────────────────────────────●────────── │
     │     │                                            │           │
     │     │◀────── bead processing continues ─────────▶│           │
     │                                                              │
     └──────────────────────────────────────────────────────────────┘
```

### Graceful Pause

Graceful pause stops at the next turn boundary, allowing Claude to complete current tool execution.

```
                       GRACEFUL PAUSE FLOW
     ┌─────────────────────────────────────────────────┐
     │                                                 │
     │  User: atari pause --graceful                   │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == IDLE:                       │    │
     │  │   -> immediately transition to PAUSED   │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == WORKING:                    │    │
     │  │   -> set gracefulPauseSignal            │    │
     │  │   -> session.RequestPause()             │    │
     │  │   -> at next turn boundary:             │    │
     │  │      - send wrap-up prompt (optional)   │    │
     │  │      - session.Stop()                   │    │
     │  │   -> PAUSED                             │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     └─────────────────────────────────────────────────┘

     Timeline (working state):
     ┌──────────────────────────────────────────────────────────────┐
     │                                                              │
     │  graceful       current turn        turn boundary            │
     │  pause          completes           -> stop session          │
     │  requested                          -> PAUSED                │
     │     │               │                    │                   │
     │     ▼               ▼                    ▼                   │
     │ ────●───────────────●────────────────────●────────────────── │
     │     │               │                    │                   │
     │     │◀── turn N ───▶│                    │                   │
     │                     │◀── (interrupted) ─▶│                   │
     │                                                              │
     │  Note: Bead is NOT marked complete - will be retried         │
     │                                                              │
     └──────────────────────────────────────────────────────────────┘
```

### Resume

Resume returns from paused state to idle, resuming work polling.

```
                          RESUME FLOW
     ┌─────────────────────────────────────────────────┐
     │                                                 │
     │  User: atari resume                             │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == PAUSED:                     │    │
     │  │   -> transition to IDLE                 │    │
     │  │   -> begin polling for beads            │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state != PAUSED:                     │    │
     │  │   -> no-op (signal ignored)             │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     └─────────────────────────────────────────────────┘

     PAUSED ──── resume ────▶ IDLE ──── bead found ────▶ WORKING
```

### Stop

Stop triggers graceful shutdown, waiting for any active session to complete.

```
                           STOP FLOW
     ┌─────────────────────────────────────────────────┐
     │                                                 │
     │  User: atari stop  (or SIGINT/SIGTERM)          │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == IDLE:                       │    │
     │  │   -> STOPPING -> STOPPED                │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == WORKING:                    │    │
     │  │   -> STOPPING                           │    │
     │  │   -> wait for session to complete       │    │
     │  │   -> STOPPED                            │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     │  ┌─────────────────────────────────────────┐    │
     │  │ IF state == PAUSED:                     │    │
     │  │   -> STOPPING -> STOPPED                │    │
     │  └─────────────────────────────────────────┘    │
     │                                                 │
     └─────────────────────────────────────────────────┘

     Timeline (working state):
     ┌──────────────────────────────────────────────────────────────┐
     │                                                              │
     │  stop            session continues           session ends    │
     │  requested       (no interruption)           -> STOPPED      │
     │     │                   │                         │          │
     │     ▼                   ▼                         ▼          │
     │ ────●───────────────────────────────────────────-─●───────── │
     │     │                                             │          │
     │     │◀────── state: STOPPING ───────────────────▶│          │
     │                                                              │
     └──────────────────────────────────────────────────────────────┘
```

## Comparison of Control Actions

| Action | Current Turn | Current Session | Bead Status | When to Use |
|--------|--------------|-----------------|-------------|-------------|
| **Pause** | Completes | Completes | Verified | Take a break, check progress |
| **Graceful Pause** | Completes | Interrupted | Not verified | Need to stop soon, preserve work |
| **Stop** | Completes | Completes | Verified | Shutdown the daemon |

## Error Handling and Recovery

### Session Failure

```
                       SESSION FAILURE FLOW
     ┌─────────────────────────────────────────────────────────────┐
     │                                                             │
     │  Session exits with error (non-zero exit code)              │
     │                                                             │
     │  ┌─────────────────────────────────────────────────────┐    │
     │  │  1. Record failure in workqueue history             │    │
     │  │  2. Apply exponential backoff                       │    │
     │  │     - Attempt 1: no backoff                         │    │
     │  │     - Attempt 2: 5s backoff                         │    │
     │  │     - Attempt 3: 10s backoff                        │    │
     │  │     - etc. (doubles each time, max 5 min)           │    │
     │  │  3. If attempts >= max_failures:                    │    │
     │  │     -> bead is ABANDONED                            │    │
     │  │     -> BeadAbandonedEvent emitted                   │    │
     │  │  4. Transition to IDLE                              │    │
     │  └─────────────────────────────────────────────────────┘    │
     │                                                             │
     └─────────────────────────────────────────────────────────────┘
```

### Session Timeout

```
                       SESSION TIMEOUT FLOW
     ┌─────────────────────────────────────────────────────────────┐
     │                                                             │
     │  Watchdog detects no activity for timeout duration          │
     │  (default: 60 minutes)                                      │
     │                                                             │
     │  ┌─────────────────────────────────────────────────────┐    │
     │  │  1. SessionTimeoutEvent emitted                     │    │
     │  │  2. session.Stop() called (kills claude process)    │    │
     │  │  3. Treated as session failure                      │    │
     │  │  4. Backoff applied, retry or abandon               │    │
     │  └─────────────────────────────────────────────────────┘    │
     │                                                             │
     │  Activity is tracked per-event:                             │
     │  - Each parsed stream-json event resets the timer           │
     │  - Watchdog checks every 10 seconds                         │
     │                                                             │
     └─────────────────────────────────────────────────────────────┘
```

### Unclosed Bead (Follow-up Session)

```
                    UNCLOSED BEAD RECOVERY FLOW
     ┌─────────────────────────────────────────────────────────────┐
     │                                                             │
     │  Session completes normally but bead is not closed          │
     │                                                             │
     │  ┌─────────────────────────────────────────────────────┐    │
     │  │  1. Run follow-up session (if enabled)              │    │
     │  │     - Reduced max_turns (default: 10)               │    │
     │  │     - Prompt: "Verify and close bead or reset"      │    │
     │  │                                                     │    │
     │  │  2. Check outcome:                                  │    │
     │  │     - Closed: record success                        │    │
     │  │     - Reset to open: record failure (retry later)   │    │
     │  │     - Still stuck: force reset to open              │    │
     │  └─────────────────────────────────────────────────────┘    │
     │                                                             │
     └─────────────────────────────────────────────────────────────┘
```

## Event Flow

### Events During a Session

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              EVENT TIMELINE                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  DrainStartEvent                                                            │
│       │                                                                     │
│       ▼                                                                     │
│  IterationStartEvent {bead_id, title, priority, attempt}                    │
│       │                                                                     │
│       ▼                                                                     │
│  SessionStartEvent {bead_id, title}                                         │
│       │                                                                     │
│       ├──▶ ClaudeTextEvent {text}           ─┐                              │
│       ├──▶ ClaudeToolUseEvent {name, input}  │                              │
│       ├──▶ ClaudeToolResultEvent {content}   ├── (repeats per turn)         │
│       ├──▶ TurnCompleteEvent {turn_number}  ─┘                              │
│       │                                                                     │
│       ▼                                                                     │
│  SessionEndEvent {session_id, num_turns, cost, duration}                    │
│       │                                                                     │
│       ▼                                                                     │
│  IterationEndEvent {bead_id, success, cost, duration}                       │
│       │                                                                     │
│       ▼                                                                     │
│  DrainStopEvent {reason}                                                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Configuration Reference

| Setting | Default | Description |
|---------|---------|-------------|
| `claude.timeout` | 60m | Idle timeout before killing session |
| `claude.max_turns` | 0 (unlimited) | Max turns per session |
| `workqueue.poll_interval` | 10s | How often to poll for ready beads |
| `backoff.initial` | 5s | Initial backoff after failure |
| `backoff.max` | 5m | Maximum backoff duration |
| `backoff.max_failures` | 3 | Failures before abandoning bead |
| `follow_up.enabled` | true | Run follow-up for unclosed beads |
| `follow_up.max_turns` | 10 | Max turns for follow-up session |
| `wrap_up.enabled` | true | Send wrap-up on graceful pause |
| `wrap_up.timeout` | 30s | Timeout for wrap-up completion |

## Related Documentation

- [Controller Component](components/controller.md) - State machine implementation
- [Session Component](components/session.md) - Claude process management
- [Workqueue Component](components/workqueue.md) - Bead selection and backoff
- [Events Component](components/events.md) - Event types and routing
- [Design Overview](DESIGN.md) - High-level architecture
