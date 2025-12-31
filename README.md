# atari

**Applied Training: Automatic Research & Implementation**

A daemon controller that orchestrates Claude Code sessions to automatically work through beads (bd) issues until all ready work is complete.

## Problem Statement

When using Claude Code with the beads issue tracker, the ideal workflow is:
1. Create beads for planned work (via `bd create` or planning sessions)
2. Have Claude automatically work through all ready beads without human intervention
3. Monitor progress in real-time with good observability
4. Survive interruptions and resume later

The current shell-script approach works but has limitations:
- No persistent state between iterations
- No real-time bead status visualization (only see changes after iteration completes)
- One fresh session per iteration (no session continuity)
- Cannot pause/resume gracefully
- Polling-only, no event-driven architecture

## Solution

A daemon controller written in Go that:
- Maintains persistent state across restarts
- Provides unified event stream (Claude activity + bd activity)
- Manages Claude Code sessions programmatically
- Offers terminal UI for monitoring
- Can be paused, resumed, and controlled externally

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         atari daemon                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ Work Queue   │    │ Session Mgr  │    │ Event Router │      │
│  │              │    │              │    │              │      │
│  │ bd ready     │───▶│ claude -p    │───▶│ Parse JSON   │      │
│  │ + state      │    │ lifecycle    │    │ Route events │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌──────────────────────────────────────────────────────────────┐
│  │                    Unified Event Stream                      │
│  │  (Claude tool calls + bd mutations + session lifecycle)      │
│  └──────────────────────────────────────────────────────────────┘
│         │                   │                   │               │
│         ▼                   ▼                   ▼               │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ Terminal UI  │    │ Log File     │    │ State File   │      │
│  │ (optional)   │    │ (JSON lines) │    │ (persist)    │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Start daemon in current directory (where .beads/ exists)
atari start

# Start with TUI monitoring
atari start --tui

# Check status
atari status

# Pause (finish current bead, then stop)
atari pause

# Resume
atari resume

# Stop immediately
atari stop
```

## Documentation

- [Design Document](docs/DESIGN.md) - Architecture and design decisions
- [Context & Research](docs/CONTEXT.md) - Background research on Claude Code and beads
- [Implementation Plan](docs/IMPLEMENTATION.md) - Phased implementation approach

## Requirements

- Go 1.25+
- Claude Code CLI (`claude` command)
- beads CLI (`bd` command)
- A project with `.beads/` initialized

## License

MIT
