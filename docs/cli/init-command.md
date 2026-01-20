# atari init Command

Initialize Claude Code configuration for use with atari and the br issue tracking system.

## Purpose

The `atari init` command configures Claude Code with the necessary:
- Rules for br issue tracking workflow
- Skills for common operations
- Settings for autonomous operation
- Session protocols for consistent behavior

This makes it easy for users to set up Claude Code to work effectively with the br system, without manually copying configuration files.

## Usage

```
atari init [flags]

Flags:
      --dry-run      Show what would be changed without making changes
      --force        Overwrite existing configuration
      --minimal      Only add essential configuration
      --global       Install to global ~/.claude/ instead of project
  -h, --help         Help for init
```

## Examples

```bash
# Preview what will be installed
atari init --dry-run

# Install configuration
atari init

# Install minimal configuration only
atari init --minimal

# Force overwrite existing files
atari init --force

# Install globally
atari init --global
```

## What Gets Installed

### Standard Installation

When you run `atari init`, it installs:

```
~/.claude/
├── rules/
│   ├── issue-tracking.md      # BR workflow patterns
│   └── session-protocol.md    # Session procedures
├── skills/
│   └── br-issue-tracking.md   # BR skill definition
└── CLAUDE.md                  # Updated with br instructions (appended)
```

### Minimal Installation (--minimal)

Only installs the essential issue-tracking rule:

```
~/.claude/
└── rules/
    └── issue-tracking.md
```

## Files Installed

### rules/issue-tracking.md

```markdown
# Issue Tracking with br

Track work with `br` for persistent context across sessions.

## Quick Commands

| Task | Command |
|------|---------|
| Find ready work | `br ready --json` |
| Start work | `br update bd-xxx --status in_progress --json` |
| Checkpoint | `br update bd-xxx --notes "COMPLETED: ..." --json` |
| Complete work | `br close bd-xxx --reason "..." --json` |
| View details | `br show bd-xxx --json` |

## Create Issue

\`\`\`bash
br create --title "Title" --description "$(cat <<'EOF'
# Description
What and why.

# Relevant files
Files from discovery.
EOF
)" --json
\`\`\`

## Notes Format

\`\`\`
COMPLETED: What was done
KEY DECISION: Why this approach
IN PROGRESS: Current state
NEXT: Immediate next step
\`\`\`

## Do NOT Close If

- Tests failing
- Implementation partial
- Unresolved errors
- Integration tests not updated

Instead: `br update bd-xxx --notes "BLOCKED: ..." --json`
```

### rules/session-protocol.md

```markdown
# Session Protocol

## Work on ONE issue at a time

1. Select highest-priority from `br ready`
2. Implement ONLY that feature
3. Commit with `/commit`
4. Close: `br close <id> --reason "..." --json`
5. Verify end-to-end
6. Move to next issue

## Quality Gates

Before committing:
- Code compiles/lints without errors
- All tests pass
- No hardcoded secrets
- Changes minimal and focused
```

### skills/br-issue-tracking.md

```markdown
# BR Issue Tracking Skill

Use the br CLI for issue tracking workflow.

## Trigger

Use when:
- Starting work on an issue
- Updating issue status
- Creating new issues
- Closing completed work

## Workflow

1. Check available work: `br ready --json`
2. Claim issue: `br update <id> --status in_progress`
3. Work on implementation
4. Checkpoint progress: `br update <id> --notes "..."`
5. Complete: `br close <id> --reason "..."`
```

## Interactive Mode

When run without `--dry-run`, the command shows an interactive summary:

```
$ atari init

Atari will install the following Claude Code configuration:

FILES TO CREATE:
  ~/.claude/rules/issue-tracking.md
    - BR workflow patterns and quick reference
    - 45 lines

  ~/.claude/rules/session-protocol.md
    - Session procedures and quality gates
    - 32 lines

  ~/.claude/skills/br-issue-tracking.md
    - Skill for br CLI operations
    - 28 lines

FILES TO MODIFY:
  ~/.claude/CLAUDE.md
    - Append br integration instructions
    - +15 lines

Proceed? [Y/n]
```

## Dry Run Output

```
$ atari init --dry-run

DRY RUN - No changes will be made

Would create: ~/.claude/rules/issue-tracking.md
--- BEGIN FILE ---
# Issue Tracking with br
...
--- END FILE ---

Would create: ~/.claude/rules/session-protocol.md
--- BEGIN FILE ---
# Session Protocol
...
--- END FILE ---

Would create: ~/.claude/skills/br-issue-tracking.md
--- BEGIN FILE ---
# BR Issue Tracking Skill
...
--- END FILE ---

Would append to: ~/.claude/CLAUDE.md
--- BEGIN APPEND ---
# BR Integration
...
--- END APPEND ---

Run without --dry-run to apply changes.
```

## Conflict Handling

If files already exist:

```
$ atari init

WARNING: The following files already exist:
  ~/.claude/rules/issue-tracking.md
  ~/.claude/rules/session-protocol.md

Options:
  1. Skip existing files (default)
  2. Backup and replace (creates .bak files)
  3. Abort

Choose [1/2/3]:
```

With `--force`, existing files are backed up and replaced:

```
$ atari init --force

Backed up: ~/.claude/rules/issue-tracking.md -> ~/.claude/rules/issue-tracking.md.bak
Created: ~/.claude/rules/issue-tracking.md
...
```

## Implementation

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize Claude Code configuration for atari",
    Long: `Initialize Claude Code with rules, skills, and settings
for use with atari and the br issue tracking system.`,
    RunE: runInit,
}

func init() {
    initCmd.Flags().Bool("dry-run", false, "Show what would be changed")
    initCmd.Flags().Bool("force", false, "Overwrite existing files")
    initCmd.Flags().Bool("minimal", false, "Install minimal configuration")
    initCmd.Flags().Bool("global", false, "Install to global ~/.claude/")

    rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
    dryRun, _ := cmd.Flags().GetBool("dry-run")
    force, _ := cmd.Flags().GetBool("force")
    minimal, _ := cmd.Flags().GetBool("minimal")
    global, _ := cmd.Flags().GetBool("global")

    // Determine target directory
    targetDir := ".claude"
    if global {
        home, _ := os.UserHomeDir()
        targetDir = filepath.Join(home, ".claude")
    }

    // Build list of files to install
    files := buildFileList(minimal)

    // Check for conflicts
    conflicts := checkConflicts(targetDir, files)

    if dryRun {
        return showDryRun(targetDir, files)
    }

    if len(conflicts) > 0 && !force {
        return handleConflicts(conflicts)
    }

    // Show summary and confirm
    if !confirmInstall(targetDir, files) {
        return nil
    }

    // Install files
    return installFiles(targetDir, files, force)
}

//go:embed templates/*
var templates embed.FS

type installFile struct {
    Path     string
    Content  string
    IsAppend bool
}

func buildFileList(minimal bool) []installFile {
    files := []installFile{
        {
            Path:    "rules/issue-tracking.md",
            Content: mustReadTemplate("templates/issue-tracking.md"),
        },
    }

    if !minimal {
        files = append(files,
            installFile{
                Path:    "rules/session-protocol.md",
                Content: mustReadTemplate("templates/session-protocol.md"),
            },
            installFile{
                Path:    "skills/br-issue-tracking.md",
                Content: mustReadTemplate("templates/br-issue-tracking.md"),
            },
            installFile{
                Path:     "CLAUDE.md",
                Content:  mustReadTemplate("templates/claude-md-append.md"),
                IsAppend: true,
            },
        )
    }

    return files
}

func installFiles(targetDir string, files []installFile, force bool) error {
    for _, f := range files {
        path := filepath.Join(targetDir, f.Path)

        // Create directory
        dir := filepath.Dir(path)
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("create directory %s: %w", dir, err)
        }

        // Handle existing files
        if _, err := os.Stat(path); err == nil {
            if force {
                // Backup existing
                backup := path + ".bak"
                os.Rename(path, backup)
                fmt.Printf("Backed up: %s -> %s\n", path, backup)
            } else if !f.IsAppend {
                fmt.Printf("Skipped (exists): %s\n", path)
                continue
            }
        }

        // Write or append
        if f.IsAppend {
            file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
            if err != nil {
                return fmt.Errorf("open %s: %w", path, err)
            }
            file.WriteString("\n\n" + f.Content)
            file.Close()
            fmt.Printf("Appended: %s\n", path)
        } else {
            if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
                return fmt.Errorf("write %s: %w", path, err)
            }
            fmt.Printf("Created: %s\n", path)
        }
    }

    fmt.Println("\nClaude Code configuration initialized successfully!")
    fmt.Println("You can now use 'atari start' to begin processing beads.")

    return nil
}
```

## Templates

Templates are embedded in the binary using Go's `embed` package:

```go
//go:embed templates/*
var templates embed.FS

func mustReadTemplate(path string) string {
    data, err := templates.ReadFile(path)
    if err != nil {
        panic(err)
    }
    return string(data)
}
```

Template files are stored in `internal/init/templates/`:

```
internal/init/templates/
├── issue-tracking.md
├── session-protocol.md
├── br-issue-tracking.md
└── claude-md-append.md
```

These templates are based on the user's existing Claude configuration patterns documented in [EXISTING_IMPLEMENTATION.md](../EXISTING_IMPLEMENTATION.md). The templates reference:
- Skills: `br-issue-tracking`, `git-commit`
- Agents: `Explore`, `Plan`
- MCPs: `codex` for verification
- Rules: Issue tracking workflow, session protocol

## Verification

After installation, users can verify with:

```bash
# Check files exist
ls -la ~/.claude/rules/
ls -la ~/.claude/skills/

# Test br integration
br ready --json
```

## Uninstallation

To remove atari configuration:

```bash
# Remove rules
rm ~/.claude/rules/issue-tracking.md
rm ~/.claude/rules/session-protocol.md

# Remove skills
rm ~/.claude/skills/br-issue-tracking.md

# Manually edit CLAUDE.md to remove br section
```

## Future Considerations

- **atari uninstall**: Automated removal of configuration
- **atari update**: Update configuration to latest version
- **Version tracking**: Track which version of config is installed
- **Custom templates**: Allow users to customize templates
- **Project-local config**: Support per-project .claude directories
