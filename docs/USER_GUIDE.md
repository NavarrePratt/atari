# Atari User Guide

A guide to using Atari for automated bead processing with Claude Code.

## Table of Contents

1. [What is Atari?](#what-is-atari)
2. [Prerequisites](#prerequisites)
3. [Quick Start](#quick-start)
4. [Basic Usage](#basic-usage)
5. [Understanding the Workflow](#understanding-the-workflow)
6. [Creating Beads with Planning Skills](#creating-beads-with-planning-skills)
7. [Configuration](#configuration)
8. [Monitoring Progress](#monitoring-progress)
9. [Observer Mode](#observer-mode)
10. [Notifications](#notifications)
11. [Troubleshooting](#troubleshooting)
12. [Best Practices](#best-practices)

---

## What is Atari?

Atari is a daemon controller that orchestrates Claude Code sessions to automatically work through your beads (bd) issues. Instead of manually starting Claude sessions and directing them to work on issues, Atari:

- Polls for available work using `bd ready`
- Spawns Claude Code sessions to work on issues
- Tracks progress and costs
- Handles failures with automatic retry and backoff
- Notifies you of important events

Think of it as a "CI for your tasks" - you create issues in bd, and Atari works through them autonomously.

---

## Prerequisites

Before using Atari, you need:

1. **Claude Code CLI** installed and authenticated
   ```bash
   claude --version
   ```

2. **Beads (bd)** installed and configured
   ```bash
   bd --version
   bd ready  # Should work without errors
   ```

3. **A project with beads** - Atari needs issues to work on
   ```bash
   cd your-project
   bd list  # Should show your issues
   ```

---

## Quick Start

### 1. Install Atari

```bash
# From source
go install github.com/yourorg/atari@latest

# Or build locally
git clone https://github.com/yourorg/atari
cd atari
go build -o atari ./cmd/atari
```

### 2. Initialize Claude Configuration

```bash
# Preview what will be installed
atari init --dry-run

# Install configuration
atari init
```

This sets up Claude Code with the rules and skills needed for bd integration.

### 3. Create Some Beads

```bash
# Create a test issue
bd create --title "Add hello world function" --description "Create a simple hello() function that returns 'Hello, World!'"
```

### 4. Start Atari

```bash
# Start in foreground with TUI
atari start

# Or start as background daemon
atari start --daemon
```

### 5. Watch It Work

Atari will:
1. Find the issue via `bd ready`
2. Spawn a Claude session
3. Claude implements the feature
4. Issue gets closed
5. Atari moves to the next issue

---

## Basic Usage

### Starting Atari

```bash
# Foreground with TUI (default)
atari start

# Background daemon
atari start --daemon

# With options
atari start --max-turns 100 --label urgent
```

### Controlling Atari

```bash
# Check status
atari status

# Pause after current bead
atari pause

# Resume
atari resume

# Stop
atari stop

# View events
atari events --follow
```

### Viewing Progress

```bash
# Current status
atari status

# Detailed statistics
atari stats

# Recent events
atari events --count 50
```

---

## Understanding the Workflow

### The Drain Loop

```
┌─────────────────────────────────────────┐
│                                         │
│  1. Check bd ready for available work   │
│              ↓                          │
│  2. Select highest priority bead        │
│              ↓                          │
│  3. Spawn Claude session                │
│              ↓                          │
│  4. Claude works on the bead            │
│     - Reads code                        │
│     - Makes changes                     │
│     - Runs tests                        │
│     - Commits with /commit              │
│     - Closes bead with bd close         │
│              ↓                          │
│  5. Session ends                        │
│              ↓                          │
│  6. Loop back to step 1                 │
│                                         │
└─────────────────────────────────────────┘
```

### What Claude Does

Each Claude session receives a prompt instructing it to:

1. Run `bd ready --json` to find work
2. Work on the highest-priority issue
3. Implement the feature or fix completely
4. Write tests if appropriate
5. Commit using `/commit`
6. Close the issue with `bd close`

### State Persistence

Atari saves state to `.atari/state.json`:
- Current bead being worked on
- Iteration count
- Cost tracking
- Bead history (attempts, failures)

If Atari crashes or is interrupted, it resumes from the saved state.

---

## Creating Beads with Planning Skills

Atari works best when beads are well-planned and properly sequenced. Three Claude Code skills help automate this:

### Available Planning Skills

| Skill | Use When | Models Used |
|-------|----------|-------------|
| `/bd-plan` | Standard planning for most tasks | Haiku + gpt-5.1-codex-mini |
| `/bd-plan-ultra` | Complex features, large refactors | Opus + gpt-5.2-codex |
| `/bd-sequence` | Order existing issues optimally | Haiku + gpt-5.1-codex-mini |

### The Planning Workflow

#### 1. Plan Your Work

Start a Claude session and describe what you want to accomplish:

```
User: I need to add user authentication to the API

Claude: /bd-plan Add user authentication with JWT tokens
```

The planning skill will:
1. **Discover**: Explore your codebase to understand architecture, patterns, and testing setup
2. **Debate**: Claude and Codex collaboratively critique and refine the plan
3. **Create Issues**: Generate well-scoped bd issues with clear acceptance criteria
4. **Add Verification**: Each issue includes exact lint/test/e2e commands for your project
5. **Create Epic**: Links all issues together for tracking

#### 2. Sequence the Issues

After planning creates issues, sequence them to set dependencies:

```
Claude: /bd-sequence
```

This skill:
1. Analyzes which files each issue will modify
2. Identifies potential merge conflicts and hidden dependencies
3. Creates an optimal linear execution order
4. Sets `bd dep add` relationships so issues complete in the right order

#### 3. Mark for Automation

If using label-based gating (recommended), add the automation label after sequencing:

```bash
# Add the label to all sequenced issues
bd list --json | jq -r '.[].id' | xargs -I{} bd update {} --labels automated
```

Now Atari will pick them up in the correct dependency order.

### Example: Full Planning Session

```
# Start planning session
User: Plan the implementation of rate limiting for API endpoints

Claude: /bd-plan-ultra Implement rate limiting for API endpoints

# Claude runs discovery, debate rounds, creates issues:
# - bd-101: Add rate limiter middleware
# - bd-102: Integrate middleware with API routes
# - bd-103: Add rate limit configuration
# - bd-104: Run full E2E test suite (final verification)

# Sequence the created issues
Claude: /bd-sequence

# Claude analyzes file overlaps, creates dependencies:
# bd-103 → bd-101 → bd-102 → bd-104

# Mark for automation (if using label filter)
User: Mark those for automation

Claude: bd update bd-101 --labels automated
        bd update bd-102 --labels automated
        bd update bd-103 --labels automated
        bd update bd-104 --labels automated

# Now start atari
$ atari start
```

### Why Sequence Matters

Without sequencing, issues might be processed in any order, leading to:
- **Merge conflicts**: Two issues modifying the same file simultaneously
- **Broken dependencies**: Issue B runs before Issue A that it depends on
- **Wasted work**: Claude has to redo work because prerequisites weren't met

The `/bd-sequence` skill prevents these by analyzing file-level dependencies and creating a linear chain where each issue waits for its predecessors.

### Preventing Race Conditions

If you create new issues while Atari is running, they could be picked up before being properly sequenced. To prevent this:

1. **Use label filtering** (recommended):
   ```yaml
   # .atari/config.yaml
   workqueue:
     label: "automated"
   ```
   New issues don't have the label, so Atari ignores them until you add it after sequencing.

2. **Pause before planning**:
   ```bash
   atari pause
   # ... run /bd-plan and /bd-sequence ...
   # ... add labels ...
   atari resume
   ```

See [workqueue.md](components/workqueue.md#race-condition-prevention) for details.

### Tips for Good Planning Sessions

**Do:**
- Provide context: "We use PostgreSQL" or "This is a REST API"
- Mention constraints: "Must be backwards compatible" or "No new dependencies"
- Reference existing patterns: "Similar to how auth works in /api/auth"

**Don't:**
- Be vague: "Make it better" - be specific about what you want
- Combine unrelated work: "Add auth AND refactor database AND update docs"
- Skip discovery: The planning skills need to explore your codebase first

---

## Configuration

### Config File

Create `.atari/config.yaml` in your project. Note: Claude model and settings come from your global Claude config (`~/.claude/settings.json`) by default.

```yaml
# Claude settings (most come from your global ~/.claude/settings.json)
claude:
  timeout: 60m                   # Session timeout (inactivity)
  # extra_args: ["--model", "sonnet"]  # Override global config if needed

# Filter beads by label
workqueue:
  label: "automated"

# Backoff for failed beads
backoff:
  initial: 1m
  max: 1h
  multiplier: 2

# Notifications (optional)
notifications:
  slack:
    enabled: true
    webhook_url: "https://hooks.slack.com/..."
    triggers:
      - iteration.end
      - error
```

See [configuration.md](config/configuration.md) for full options.

### Environment Variables

```bash
export ATARI_LABEL=automated
export ATARI_SLACK_WEBHOOK="https://..."
export ATARI_NO_TUI=1  # Disable TUI
```

Note: Claude model and settings come from your global Claude config (`~/.claude/settings.json`).

### Custom Prompts

Create `.atari/prompt.txt` with custom instructions:

```
Run "bd ready --json" to find work.

Special instructions for this project:
- Always run `make lint` before committing
- Use conventional commit messages
- Add tests for any new functions

{{.DefaultPrompt}}
```

---

## Monitoring Progress

### Terminal UI

The TUI shows:
- Current status (idle, working, paused)
- Current bead being worked on
- Progress statistics
- Live event feed
- Keyboard controls

```
┌─ ATARI ─────────────────────────────────────────────┐
│ Status: WORKING                    Cost: $2.35      │
│ Current: bd-042 "Fix auth bug"     Turns: 42        │
│ Progress: 4 completed, 1 failed                     │
├─ Events ────────────────────────────────────────────┤
│ 14:23:45 $ go test ./...                            │
│ 14:23:50 ✓ bd-042 closed                            │
│ 14:23:51 BEAD bd-043 "Add rate limiting"            │
│ ...                                                 │
├─────────────────────────────────────────────────────┤
│ [p] pause  [r] resume  [q] quit  [↑↓] scroll        │
└─────────────────────────────────────────────────────┘
```

### Log Files

Logs are written to `.atari/atari.log` in JSON lines format:

```bash
# View recent logs
tail -f .atari/atari.log | jq .

# Filter for errors
cat .atari/atari.log | jq 'select(.type == "error")'

# Calculate total cost
cat .atari/atari.log | jq -s '[.[] | select(.type == "session.end")] | map(.total_cost_usd) | add'
```

### Status Command

```bash
$ atari status

Status: WORKING
Current: bd-042 "Fix auth bug"
Uptime: 2h15m

Statistics:
  Beads completed: 12
  Beads failed: 1
  Total cost: $5.42
  Total turns: 156
```

---

## Observer Mode

Observer Mode provides an interactive Q&A pane for asking questions about drain activity while Atari is running. Think of it as a "what's happening?" assistant.

### What Can Observer Mode Do?

- **Real-time understanding**: Ask what Claude is currently doing
- **Intervention guidance**: Get advice on whether to pause and intervene
- **Context from events**: Observer sees the event history and current state
- **Follow-up questions**: Ask related questions without repeating context

### Using Observer Mode

Toggle the observer pane in the TUI with the `o` key:

```
┌─ ATARI ──────────────────────────────────────────────────────────────┐
│ Status: WORKING                                    Cost: $2.35       │
│ Current: bd-042 "Fix auth bug"                     Turns: 42         │
├─ Events ─────────────────────────┬─ Observer (haiku) ── $0.03 ───────┤
│ [14:02:13] Tool: Bash "Run tests"│ > Why did Claude run tests twice?  │
│ [14:02:14] Result: "PASS ok..."  │                                    │
│ [14:02:15] Claude: "Tests pass." │ The tests were run twice because   │
│ [14:02:16] Tool: Edit types.go   │ the first run had a flaky failure  │
│                                  │ in test_auth.go. Claude retried to │
│                                  │ confirm it was genuine.            │
├──────────────────────────────────┴────────────────────────────────────┤
│ [o] observer  [p] pause  [r] resume  [q] quit  [Tab] focus            │
└───────────────────────────────────────────────────────────────────────┘
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `o` | Toggle observer pane |
| `O` | Focus observer (fullscreen) |
| `Tab` | Switch focus between events and observer |
| `Enter` | Send question (when observer focused) |
| `Esc` | Close observer pane or exit focus mode |

### Example Questions

- "What is Claude doing right now?"
- "Why did it run the tests twice?"
- "Summarize what's happened so far"
- "Should I pause and intervene?"
- "What error caused the last retry?"
- "Is the current approach likely to succeed?"

### Observer Configuration

Configure observer in `.atari/config.yaml`:

```yaml
observer:
  enabled: true        # Enable observer mode in TUI
  model: haiku         # Model for observer queries (fast, low cost)
  recent_events: 20    # Events to include for current bead
  show_cost: true      # Display observer session cost
  layout: horizontal   # "horizontal" (side-by-side) or "vertical" (stacked)
```

### Cost Considerations

Observer uses Haiku by default, which is fast and inexpensive for Q&A. The observer cost is shown separately from the main drain cost:

- Observer queries typically cost $0.01-0.05 each
- Change `observer.model` to `sonnet` for deeper analysis (higher cost)
- Observer sessions are independent of drain sessions

### When to Use Observer

**Good use cases:**
- Understanding what's happening in real-time
- Deciding whether to pause and manually intervene
- Quick clarification during active work

**Better done elsewhere:**
- Post-hoc investigation (use normal Claude session with `bd show`)
- Deep analysis of past sessions (use log files directly)
- Complex debugging (pause drain and use normal Claude session)

---

## Notifications

### Setting Up Slack

1. Create a Slack webhook at https://api.slack.com/apps
2. Add to config:

```yaml
notifications:
  slack:
    enabled: true
    webhook_url: "https://hooks.slack.com/services/XXX/YYY/ZZZ"
    channel: "#atari-notifications"
    triggers:
      - iteration.end
      - error
      - drain.stop
```

### Setting Up IFTTT

1. Create an IFTTT webhook applet
2. Get your webhook key from https://ifttt.com/maker_webhooks
3. Add to config:

```yaml
notifications:
  ifttt:
    enabled: true
    key: "your-webhook-key"
    event_name: "atari_notification"
    triggers:
      - iteration.end
      - error
```

### Trigger Events

| Event | When |
|-------|------|
| `iteration.end` | A bead completes (success or failure) |
| `error` | Any error occurs |
| `bead.abandoned` | A bead hits max_failures and is abandoned |
| `drain.start` | Atari starts |
| `drain.stop` | Atari stops |
| `drain.pause` | Atari pauses |
| `session.end` | Claude session ends |

For overnight runs, configure notifications for `error`, `bead.abandoned`, and `drain.stop` to catch problems early.

---

## Troubleshooting

### Atari Won't Start

**"Daemon already running"**
```bash
atari stop
atari start
```

**"bd ready failed"**
```bash
# Check bd is working
bd ready --json
bd daemon  # May need to start daemon
```

### Beads Not Being Processed

**Check if beads are ready:**
```bash
bd ready --json
```

**Check if beads match your label filter:**
```bash
# If using --label, verify beads have that label
bd list --label automated
```

**Check for beads in backoff:**
```bash
atari stats  # Shows beads in backoff
```

### Claude Sessions Failing

**Check the log for errors:**
```bash
cat .atari/atari.log | jq 'select(.type == "error")' | tail -20
```

**Common issues:**
- Tests failing - Claude will close beads even if tests fail; add stricter instructions
- Timeout - increase `--max-turns` or simplify the bead
- Permission errors - ensure Claude has the tools it needs

### High Costs

**Monitor costs:**
```bash
atari stats  # Shows total and average cost
```

**Reduce costs:**
- Use `--model sonnet` for simpler tasks
- Reduce `--max-turns`
- Break large beads into smaller ones
- Add more specific instructions to reduce exploration

### State File Corrupted

```bash
# Backup and reset
mv .atari/state.json .atari/state.json.bak
atari start
```

---

## Best Practices

### Writing Good Beads

**Do:**
- Clear, specific titles: "Add rate limiting to /api/users endpoint"
- Detailed descriptions with context
- Include relevant file paths
- Specify expected behavior

**Don't:**
- Vague titles: "Fix the bug"
- Multi-part tasks: "Refactor auth and add logging and update docs"
- Tasks requiring external access: "Deploy to production"

### Organizing Work

- Use labels to categorize beads (`bug`, `feature`, `chore`)
- Set appropriate priorities (P0 for critical, P3 for backlog)
- Create dependencies between related beads
- Break large tasks into smaller beads

### Monitoring Runs

- Start with `atari start` (foreground) to watch initial runs
- Use `--daemon` for long-running sessions
- Set up notifications for errors and completions
- Review `.atari/atari.log` periodically

### Cost Management

- Set `--max-turns` to limit runaway sessions
- Use `sonnet` model for routine tasks
- Review stats regularly: `atari stats`
- Add spending alerts via notifications

### Handling Failures

- Check why beads are failing (logs, bd notes)
- Update bead descriptions with more context
- Consider if the task is too complex for automation
- Use `bd update` to add notes for Claude

---

## Getting Help

- **Documentation**: See the `docs/` directory
- **Issues**: Report bugs at https://github.com/yourorg/atari/issues
- **Beads CLI**: Run `bd help` for bd documentation
- **Claude Code**: Run `claude --help` for Claude documentation
