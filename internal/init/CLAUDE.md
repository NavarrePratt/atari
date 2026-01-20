# internal/init

Implements the `atari init` command that bootstraps Claude Code configuration.

## Overview

This package provides functionality to install rules, skills, and configuration for using atari with the br issue tracking system.

## Files

- `init.go` - Core logic: Run(), BuildFileList(), checkFileStatuses(), installFiles()
- `diff.go` - Simple unified diff implementation (no external dependencies)
- `templates.go` - Embedded template filesystem with MustReadTemplate()
- `templates/` - Markdown template files embedded at compile time

## Usage

```go
opts := initcmd.Options{
    DryRun:  false,
    Force:   false,
    Minimal: false,
    Global:  false,
}
result, err := initcmd.Run(opts)
```

## Options

- `DryRun` - Show what would be changed without making changes
- `Force` - Overwrite files with changes (no backup files created)
- `Minimal` - Install only essential rules (just issue-tracking.md)
- `Global` - Install to ~/.claude/ instead of ./.claude/ (shows git backup tip)
- `Writer` - Custom output writer (defaults to os.Stdout)

## Directory Structure Created

```
.claude/
  rules/
    issue-tracking.md      # Always installed
    session-protocol.md    # Unless --minimal
  skills/
    issue-tracking.md      # Unless --minimal
  commands/
    issue-create.md        # Unless --minimal
    issue-plan.md          # Unless --minimal
    issue-plan-ultra.md    # Unless --minimal
    issue-plan-user.md     # Unless --minimal
  CLAUDE.md                # Appended, never overwritten
```

## Behavior

- Creates directories as needed
- Compares content of existing files to detect actual changes
- Unchanged files show "Already up to date" and don't require --force
- Files with changes show unified diff and require --force to overwrite
- With --force, directly overwrites changed files (no backup files)
- With --global, shows tip about backing up ~/.claude with git

### CLAUDE.md Managed Section

CLAUDE.md uses a managed section approach with XML-style markers:
```markdown
<atari-managed>
# BR Integration
...content...
</atari-managed>
```

Behavior:
- If markers exist: replaces content between them (idempotent)
- If no markers: appends the managed section to existing content
- User content before/after markers is preserved
- Running init twice produces identical results (no duplication)
