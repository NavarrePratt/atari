# Getting Started with Atari

A quick guide to installing atari and running your first automated session.

## What is Atari?

Atari is a daemon that runs Claude Code sessions to work through beads (issues) automatically. Instead of manually starting Claude sessions for each task, atari:

- Polls for available beads using `br ready`
- Spawns Claude Code sessions to implement each bead
- Tracks progress, costs, and failures
- Handles retries with exponential backoff

The benefit: hands-off task automation. Create beads for your planned work, start atari, and let it process them autonomously.

## Prerequisites

Before using atari, you need:

**1. Claude Code CLI** - Installed and authenticated

```bash
claude --version
```

If not installed, see [Claude Code documentation](https://docs.anthropic.com/en/docs/claude-code).

**2. beads_rust (br)** - Issue tracking CLI

```bash
br --version
br ready  # Should work without errors
```

If not installed: `cargo install beads_rust` or see [beads_rust on GitHub](https://github.com/Dicklesworthstone/beads_rust).

**3. A project with beads initialized**

```bash
cd your-project
br init   # Creates .beads/ directory
br list   # Should show (empty) list
```

## Installation

Clone the repository and build with [mise](https://mise.jdx.dev/):

```bash
git clone https://github.com/navarrepratt/atari
cd atari
mise run build
```

To install to your Go bin directory:

```bash
mise run install
```

Verify the installation:

```bash
atari version
```

## First-time Setup

Run `atari init` to install Claude Code rules and skills for bead integration:

```bash
# Preview what will be installed
atari init --dry-run

# Install configuration
atari init
```

This creates:

- `.claude/rules/issue-tracking.md` - br CLI patterns
- `.claude/rules/session-protocol.md` - Session procedures
- `.claude/skills/issue-tracking.md` - Issue tracking skill
- `.claude/skills/issue-create/SKILL.md` - Quick bead creation
- `.claude/skills/issue-plan/SKILL.md` - AI debate planning (Claude haiku + Codex mini)
- `.claude/skills/issue-plan-codex/SKILL.md` - Thorough planning (Claude opus + Codex gpt-5.2)
- `.claude/skills/issue-plan-user/SKILL.md` - Interactive planning via user interview
- `.claude/skills/issue-plan-hybrid/SKILL.md` - User interview + Codex review
- `.claude/CLAUDE.md` - Bead integration instructions

### Minimal Setup

For a lighter installation with only essential rules:

```bash
atari init --minimal
```

This installs just `issue-tracking.md` - useful if you want to add skills manually later.

### Global Installation

To install rules globally (for all projects):

```bash
atari init --global
```

## Your First Run

### 1. Create a Test Bead

Open a Claude Code session in your project and ask it to create a simple bead:

```
You: Add a hello world function to this project

Claude: I'll create a bead for this task.

You: /issue-create
```

The `/issue-create` command (installed by `atari init`) will have Claude create a well-formed bead with a clear description. You'll see output like:

```
Created bd-xxx: Add hello world function
```

This is the recommended workflow: describe what you want in a Claude session, then use `/issue-create` to capture it as a bead for atari to process.

### 2. Start Atari (in a separate terminal)

```bash
atari start
```

The TUI shows current status, the active bead, and a live event feed.

### 3. Watch It Work

Atari will:
1. Find the bead via `br ready`
2. Spawn a Claude Code session
3. Claude implements the function
4. Claude commits the change using `/commit`
5. Claude closes the bead with `br close`
6. Atari checks for more work (none left, so it idles)

### 4. Verify Completion

Check that the bead was completed:

```bash
br list --status closed
```

You should see your "Add hello world function" bead with status `closed`.

## Basic Controls

While atari is running:

| Key | Action |
|-----|--------|
| `p` | Pause after current bead |
| `r` | Resume |
| `q` | Quit |

Or use CLI commands:

```bash
atari status   # Check current state
atari pause    # Pause after current bead
atari resume   # Resume processing
atari stop     # Stop immediately
```

## Running as a Daemon

For long-running sessions:

```bash
# Start in background
atari start --daemon

# Check status
atari status

# View events
atari events --follow

# Stop when done
atari stop
```

## Next Steps

- [Workflow Guide](workflow.md) - Two-terminal planning workflow
- [Configuration Reference](config/configuration.md) - All configuration options
- [TUI Guide](tui.md) - Terminal UI features and keybinds

### Creating Well-Planned Work

For best results, beads should be well-scoped and properly sequenced. See the [Workflow Guide](workflow.md) for planning workflows using `/issue-plan-user` and related skills.
