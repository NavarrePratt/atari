# Terminal User Interface (TUI)

This guide covers the Atari TUI: its layout, panes, navigation, and keybinds.

## Overview

The TUI shows drain activity across three panes:

- **Events pane**: Live feed of Claude activity, tool calls, and results
- **Observer pane**: Q&A about what's happening (uses a separate Claude session)
- **Graph pane**: Bead dependency visualization

Each pane can be toggled on/off, focused independently, or expanded to fullscreen.

## Layout

The TUI uses a split layout with a shared header and footer:

```
+------------------------------------------------------------------+
| WORKING (epic: bd-xxx)                              $2.3456      |
| bead: bd-042 - Fix auth bug [5m 23s, turn 12]                    |
| turns: 156  total: 2h 15m  completed: 4  failed: 1  abandoned: 0 |
+------------------+-------------------+---------------------------+
| Events           | Observer (haiku)  | Graph                     |
|                  |                   |                           |
| 14:23:45 Tool:   | You: What's it    | bd-001 [epic] Auth       |
|   Bash "make"    |   doing now?      |   bd-002 Add login       |
| 14:23:50 Result: |                   |   bd-003 Add logout  *   |
|   PASS           | Claude: Claude is |   bd-004 Add tests       |
|                  |   running tests   |                           |
+------------------+-------------------+---------------------------+
| p: pause  e/o/b: panels  E/O/B: fullscreen  tab: switch  q: quit |
+------------------------------------------------------------------+
```

**Header** shows:
- Status (IDLE, WORKING, PAUSED, STOPPED)
- Epic filter if set
- Cumulative cost
- Active bead with elapsed time and turn count
- Session statistics

**Footer** shows context-sensitive keybind hints.

### Layout Modes

Configure layout in `.atari/config.yaml`:

```yaml
observer:
  layout: horizontal  # "horizontal" (side-by-side) or "vertical" (stacked)
```

## Panes

### Events Pane

The events pane shows a live feed of drain activity:

- **Tool calls**: Commands Claude executes (Bash, Read, Edit, etc.)
- **Tool results**: Output from tool calls
- **Claude text**: Claude's reasoning and commentary
- **Session events**: Start, end, timeout
- **Bead events**: Created, updated, closed, abandoned

Events are timestamped (HH:MM:SS) and color-coded by type:
- Tool calls and results: cyan
- Session events: blue
- Bead status changes: magenta
- Errors: red

The events pane auto-scrolls to show new content. Scrolling manually disables auto-scroll until you return to the bottom.

### Observer Pane

Observer mode lets you ask questions about current activity without interrupting the drain. It runs a separate Claude session (Haiku by default) with access to recent events and current state.

**Use cases:**
- Understanding what Claude is doing right now
- Deciding whether to pause and intervene
- Quick clarification during active work

**Vim-style modes:**
- **Normal mode**: Navigate history, submit questions
- **Insert mode**: Type your question

The mode indicator shows `[NORMAL]` or `[INSERT]` in the status bar.

**Example questions:**
- "What is Claude doing right now?"
- "Why did it run the tests twice?"
- "Summarize what's happened so far"
- "Should I pause and intervene?"
- "What error caused the last retry?"

**Cost considerations:**
Observer uses Haiku by default, which is fast and inexpensive. Queries typically cost $0.01-0.05 each. Change `observer.model` to `sonnet` for deeper analysis (higher cost).

```yaml
observer:
  enabled: true
  model: haiku         # or "sonnet" for deeper analysis
  recent_events: 20    # Events included in context
  show_cost: true      # Display observer cost
```

### Graph Pane

The graph pane visualizes bead dependencies as a tree structure.

**Views** (cycle with `a`):
- **Active**: Open and in-progress beads
- **Backlog**: Deferred and low-priority beads
- **Closed**: Completed beads

**Node display:**
- The active bead is highlighted with an asterisk (`*`)
- Failed/abandoned beads show workqueue status
- Epics can be collapsed/expanded
- Out-of-scope beads (outside epic filter) are dimmed

**Density levels** (cycle with `d`):
- **Minimal**: ID only
- **Compact**: ID + truncated title
- **Standard**: ID + title + status
- **Verbose**: Full details including priority

**Detail view:**
Press `Enter` on a node to open inline detail view showing:
- Full title and description
- Labels, dependencies, dependents
- Notes and metadata

Press `Enter` again for fullscreen modal, or `Esc` to return to graph.

## Navigation

### Focus Cycling

- **Tab**: Cycle focus between open panes (Events -> Observer -> Graph -> Events)
- Focused pane has a highlighted border
- Only the focused pane receives keyboard input (except global keys)

### Fullscreen Mode

- **E**: Events fullscreen
- **O**: Observer fullscreen
- **B**: Graph fullscreen
- **Esc**: Exit fullscreen

In fullscreen mode, the selected pane fills the terminal. The header stays visible.

### Closing Panes

When all panes are closed, the TUI shows a minimal "monitoring only" view with just the header. Use `e`, `o`, or `b` to reopen panes.

## Keybind Reference

### Global Keys

These keys work regardless of which pane is focused.

| Key | Action |
|-----|--------|
| `ctrl+c` | Quit (or cancel query if observer is loading) |
| `tab` | Cycle focus between open panes |
| `esc` | Exit fullscreen, clear input, or close pane |
| `e` | Toggle events pane |
| `o` | Toggle observer pane |
| `b` | Toggle graph (bead) pane |
| `E` | Toggle events fullscreen |
| `O` | Toggle observer fullscreen |
| `B` | Toggle graph fullscreen |

### Control Keys

These keys are blocked when the observer pane is focused (to allow typing).

| Key | Action |
|-----|--------|
| `q` | Quit |
| `p` | Pause drain |
| `r` | Resume drain |

### Events Pane

| Key | Action |
|-----|--------|
| `up`, `k` | Scroll up |
| `down`, `j` | Scroll down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |

### Observer Pane

The observer pane uses vim-style modes.

**Normal Mode**

| Key | Action |
|-----|--------|
| `i` | Enter insert mode |
| `enter` | Submit question |
| `ctrl+c` | Cancel in-progress query |
| `esc` | Clear error, clear input, or close pane |
| `up`, `k` | Scroll history up |
| `down`, `j` | Scroll history down |
| `pgup` | Half page up |
| `pgdown` | Half page down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |

**Insert Mode**

| Key | Action |
|-----|--------|
| `esc` | Exit insert mode |
| `enter` | Submit question |
| `ctrl+c` | Cancel in-progress query |

### Graph Pane

| Key | Action |
|-----|--------|
| `up`, `k` | Navigate to previous node |
| `down`, `j` | Navigate to next node |
| `left`, `h` | Navigate to parent node |
| `right`, `l` | Navigate to first child |
| `a` | Cycle through Active/Backlog/Closed views |
| `c` | Collapse/expand selected epic |
| `d` | Cycle density level |
| `R` | Refresh graph data |
| `enter` | Open detail view (press again for fullscreen modal) |
| `esc` | Close detail view, clear error, or close pane |

### Detail Modal

| Key | Action |
|-----|--------|
| `esc`, `enter`, `q` | Close modal |
| `up`, `k` | Scroll up |
| `down`, `j` | Scroll down |
| `home`, `g` | Scroll to top |
| `end`, `G` | Scroll to bottom |

## Configuration

Full TUI configuration options:

```yaml
# Observer pane settings
observer:
  enabled: true              # Enable observer in TUI
  model: haiku               # Model for queries (haiku or sonnet)
  recent_events: 20          # Events included for context
  show_cost: true            # Display query cost
  layout: horizontal         # horizontal or vertical

# Graph pane settings
graph:
  enabled: true              # Enable graph in TUI
  density: standard          # minimal, compact, standard, verbose
  auto_refresh_interval: 5s  # Auto-refresh rate
```

<!-- Screenshots placeholder: Add TUI screenshots here -->
