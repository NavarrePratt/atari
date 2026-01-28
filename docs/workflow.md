# Atari Workflow Guide

This guide covers the recommended workflow for using atari: planning in one terminal, automation in another.

## The Two-Terminal Workflow

The most effective way to use atari separates planning from execution:

```
Terminal 1: Planning                    Terminal 2: Automation
┌─────────────────────────┐            ┌─────────────────────────┐
│ Claude Code session     │            │ atari start             │
│                         │            │                         │
│ - Interview for reqs    │   beads    │ - Watches for beads     │
│ - Create beads          │ ─────────> │ - Spawns Claude sessions│
│ - Set dependencies      │            │ - Implements each bead  │
│ - Review and adjust     │            │ - Closes on completion  │
└─────────────────────────┘            └─────────────────────────┘
     Human-guided                           Fully automated
```

**Why this separation?**

- **Human stays in control of planning**: You decide what gets built and in what order
- **Automation handles implementation**: Once planned, beads are processed without intervention
- **Iterative refinement**: You can add more beads while atari works through existing ones
- **Clear boundaries**: Planning requires judgment; implementation follows the plan

## Planning with /issue-plan-user

The `/issue-plan-user` skill conducts a structured interview to flesh out your requirements before creating beads.

### What It Does

1. **Asks clarifying questions** about your goals, constraints, and preferences
2. **Explores the codebase** to understand existing patterns
3. **Proposes a plan** broken into well-scoped beads
4. **Creates beads** after you approve the plan
5. **Sets up dependencies** so beads execute in the right order

### When to Use It

Use `/issue-plan-user` when you have a feature or task in mind but haven't fully specified the details. The interview process helps surface requirements you might not have considered.

**Good candidates:**
- "I want to add authentication to the API"
- "We need better error handling"
- "The dashboard should show usage metrics"

**Not needed for:**
- Simple, well-defined tasks: "Fix the typo in README.md"
- Tasks you've already planned: Use `br create` directly

### Example Session

```
Terminal 1 (Claude Code):

User: /issue-plan-user Add rate limiting to API endpoints

Claude: I'll help you plan rate limiting. Let me ask a few questions:

1. Which endpoints need rate limiting? All of them, or specific ones?

User: Just the public API endpoints under /api/v1/

Claude: Got it. What rate limits are you thinking?

User: 100 requests per minute per IP for anonymous, 1000 for authenticated

Claude: Should rate limit info be included in response headers?

User: Yes, standard headers like X-RateLimit-Remaining

Claude: Based on your codebase, I see you're using Chi router and Redis.
        Here's my proposed plan:

        1. bd-101: Add rate limiter middleware using go-redis
        2. bd-102: Configure rate limits in config.yaml
        3. bd-103: Apply middleware to /api/v1 routes
        4. bd-104: Add rate limit response headers
        5. bd-105: Add integration tests for rate limiting

        Should I create these beads?

User: Yes, create them

Claude: Created 5 beads with dependencies:
        bd-102 -> bd-101 -> bd-103 -> bd-104 -> bd-105
```

### Other Planning Skills

| Skill | Use Case |
|-------|----------|
| `/issue-plan-user` | Interview-driven planning (recommended) |
| `/issue-plan` | Quick planning without interview |
| `/issue-plan-codex` | Complex features needing deep analysis |
| `/issue-create` | Create a single bead directly |

For most planning work, start with `/issue-plan-user`. It catches requirements gaps early.

## Creating Beads

### How /issue-plan-user Creates Beads

The planning skill uses `br create` to create beads with:

- **Clear titles**: Imperative voice, 50 chars max
- **Detailed descriptions**: Context, relevant files, acceptance criteria
- **Verification commands**: Specific lint/test commands for your project
- **Deferred status**: Prevents pickup until dependencies are set

### Deferred Status for Batch Creation

When creating multiple beads, they start in `deferred` status:

```bash
# Planning skill creates beads as deferred
br create --title "Add middleware" --status deferred --description "..."
br create --title "Configure limits" --status deferred --description "..."

# After setting dependencies, publish them
br update bd-101 --status open
br update bd-102 --status open
```

This prevents atari from picking up beads before the plan is complete.

### Dependencies Between Beads

Dependencies ensure beads execute in the correct order:

```bash
# bd-103 depends on bd-102 (102 must complete before 103 starts)
br dep add bd-103 bd-102 --type blocks
```

The planning skill sets these automatically based on file-level dependencies and logical ordering.

### Publishing Beads

After planning is complete:

1. **Review the plan**: `br list --status deferred`
2. **Check dependencies**: `br show bd-xxx` for each bead
3. **Publish**: `br update bd-xxx --status open` for each bead

Or use label-based gating:

```bash
# Add automation label to all planned beads
br update bd-101 --add-label automated
br update bd-102 --add-label automated
# ...
```

Then configure atari to only process labeled beads:

```yaml
# .atari/config.yaml
workqueue:
  label: "automated"
```

## Running Atari

### Starting the TUI

In your second terminal:

```bash
atari start
```

The TUI shows:
- Current status (idle, working, paused)
- Active bead and progress
- Live event feed from Claude sessions

### Basic Controls

| Key | Action |
|-----|--------|
| `p` | Pause after current bead completes |
| `r` | Resume processing |
| `q` | Quit atari |

See the [TUI Guide](tui.md) for the full keybind reference.

### What Happens When a Bead Completes

1. Claude commits changes using `/commit`
2. Claude closes the bead with `br close --reason "..."`
3. Atari checks `br ready` for the next bead
4. If another bead is ready, a new Claude session starts
5. If no beads are ready, atari idles until work appears

### Pausing and Resuming

**Pause** when you need to:
- Plan more work without race conditions
- Manually intervene on a problematic bead
- Take a break from monitoring

```bash
# From TUI: press 'p'
# Or from CLI:
atari pause
```

Atari finishes the current bead, then waits.

**Resume** to continue:

```bash
# From TUI: press 'r'
# Or from CLI:
atari resume
```

## Important Limitation: Single Worker

Atari processes **one bead at a time**. There is no parallel execution.

This means:
- Beads are processed sequentially in dependency/priority order
- A slow bead blocks subsequent beads
- Total time = sum of individual bead times

**Why single worker?**

- **Avoids conflicts**: Multiple Claude sessions editing the same files would collide
- **Simpler state**: No merge resolution or coordination needed
- **Predictable costs**: One session at a time means predictable spend

**Working with the constraint:**

- Break large tasks into smaller beads (faster individual completion)
- Set priorities so important beads run first
- Use dependencies to ensure logical ordering
- Monitor progress in the TUI

## Putting It Together

A typical session:

```bash
# Terminal 1: Start Claude Code for planning
claude

> /issue-plan-user Add user authentication

# ... interview happens, beads created ...

> # Planning complete, beads are ready
```

```bash
# Terminal 2: Start atari to process beads
atari start

# Watch progress in TUI
# Press 'p' to pause if needed
# Press 'q' when done
```

```bash
# Terminal 1: Continue planning while atari works
> /issue-plan-user Add API documentation

# More beads created, atari picks them up automatically
```

This workflow lets you plan at your own pace while automation handles implementation.
