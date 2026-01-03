# internal/init

Implements the `atari init` command that bootstraps Claude Code configuration.

## Overview

This package provides functionality to install rules, skills, and configuration for using atari with the bd issue tracking system.

## Files

- `init.go` - Core logic: Run(), BuildFileList(), checkConflicts(), installFiles()
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
- `Force` - Overwrite existing files (creates timestamped backups)
- `Minimal` - Install only essential rules (just issue-tracking.md)
- `Global` - Install to ~/.claude/ instead of ./.claude/
- `Writer` - Custom output writer (defaults to os.Stdout)

## Directory Structure Created

```
.claude/
  rules/
    issue-tracking.md      # Always installed
    session-protocol.md    # Unless --minimal
  skills/
    bd-issue-tracking.md   # Unless --minimal
  commands/
    bd-plan.md             # Unless --minimal
    bd-plan-ultra.md       # Unless --minimal
  CLAUDE.md                # Appended, never overwritten
```

## Behavior

- Creates directories as needed
- CLAUDE.md is always appended, never replaced
- Without --force, skips existing files with a warning
- With --force, creates timestamped backups before replacing
