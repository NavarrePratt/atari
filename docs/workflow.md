# Atari Workflow Guide

This guide covers the recommended workflow for using atari: planning in one terminal, automation in another.

## The two-terminal workflow

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

### What it does

1. **Asks clarifying questions** about your goals, constraints, and preferences
2. **Explores the codebase** to understand existing patterns
3. **Proposes a plan** broken into well-scoped beads
4. **Creates beads** after you approve the plan
5. **Sets up dependencies** so beads execute in the right order

### When to use it

Use `/issue-plan-user` when you have a feature or task in mind but haven't fully specified the details. The interview process helps surface requirements you might not have considered.

**Good candidates:**
- "I want to add authentication to the API"
- "We need better error handling"
- "The dashboard should show usage metrics"

**For simpler tasks:**
- Use `/issue-create` for single, well-defined tasks: "Fix the typo in README.md"
- Always use a planning skill rather than `br create` directly - skills ensure beads have proper verification instructions that atari needs

### Example session

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

### Other planning skills

| Skill | Use Case |
|-------|----------|
| `/issue-plan-user` | Interview-driven planning: asks probing questions directly |
| `/issue-plan` | AI debate: Claude (haiku) + Codex (mini) refine the plan |
| `/issue-plan-codex` | AI debate: Claude (opus) + Codex (gpt-5.2) for complex work |
| `/issue-plan-hybrid` | User interview + Codex review iterates until consensus |
| `/issue-create` | Quick single-bead creation with proper verification setup |

For most planning work, start with `/issue-plan-user` or `/issue-plan-hybrid`. They catch requirements gaps early through direct user input.

## How planning skills create beads

**Always use a planning skill** (`/issue-plan-user`, `/issue-plan`, `/issue-plan-hybrid`, `/issue-plan-codex`, or `/issue-create`) rather than running `br create` directly. Planning skills ensure beads include:

- **Verification commands**: Specific lint/test commands that atari uses to validate completion
- **Structured descriptions**: Context and acceptance criteria in a format atari understands
- **Proper sequencing**: Dependencies set correctly for execution order

Without these, atari cannot properly verify that work is complete before closing beads.

### What planning skills generate

The planning skills use `br create` internally to create beads with:

- **Clear titles**: Imperative voice, 50 chars max
- **Detailed descriptions**: Context, relevant files, acceptance criteria
- **Verification commands**: Specific lint/test commands for your project
- **Deferred status**: Prevents pickup until dependencies are set

### Deferred status for batch creation

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

### Dependencies between beads

Dependencies ensure beads execute in the correct order:

```bash
# bd-103 depends on bd-102 (102 must complete before 103 starts)
br dep add bd-103 bd-102 --type blocks
```

The planning skill sets these automatically based on file-level dependencies and logical ordering.

### Publishing beads

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

## Running atari

### Starting the TUI

In your second terminal:

```bash
atari start
```

The TUI shows:
- Current status (idle, working, paused)
- Active bead and progress
- Live event feed from Claude sessions

### Basic controls

| Key | Action |
|-----|--------|
| `p` | Pause after current bead completes |
| `r` | Resume processing |
| `q` | Quit atari |

See the [TUI Guide](tui.md) for the full keybind reference.

### What happens when a bead completes

1. Claude commits changes using `/commit`
2. Claude closes the bead with `br close --reason "..."`
3. Atari checks `br ready` for the next bead
4. If another bead is ready, a new Claude session starts
5. If no beads are ready, atari idles until work appears

### Pausing and resuming

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

## Important limitation: single worker

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

## Putting it together

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

You plan, atari implements.
