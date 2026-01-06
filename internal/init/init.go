package initcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Options configures the init command behavior.
type Options struct {
	DryRun  bool
	Force   bool
	Minimal bool
	Global  bool
	Writer  io.Writer // Output writer (defaults to os.Stdout)
}

// InstallFile represents a file to be installed.
type InstallFile struct {
	Path     string // Relative path within .claude directory
	Content  string // File content
	IsAppend bool   // If true, append to existing file instead of replacing
}

// Result contains the outcome of the init operation.
type Result struct {
	TargetDir  string
	Created    []string
	Appended   []string
	Skipped    []string
	Unchanged  []string
	Overwritten []string
}

// FileStatus represents the status of a file to be installed.
type FileStatus struct {
	Path      string // Relative path within .claude directory
	Exists    bool   // True if file exists
	Unchanged bool   // True if existing content matches new content
	Diff      string // Unified diff if changed (empty if unchanged or new)
}

// BuildFileList returns the list of files to install based on options.
func BuildFileList(minimal bool) []InstallFile {
	files := []InstallFile{
		{
			Path:    "rules/issue-tracking.md",
			Content: MustReadTemplate("issue-tracking.md"),
		},
	}

	if !minimal {
		files = append(files,
			InstallFile{
				Path:    "rules/session-protocol.md",
				Content: MustReadTemplate("session-protocol.md"),
			},
			InstallFile{
				Path:    "skills/bd-issue-tracking.md",
				Content: MustReadTemplate("bd-issue-tracking.md"),
			},
			InstallFile{
				Path:    "commands/bd-create.md",
				Content: MustReadTemplate("bd-create.md"),
			},
			InstallFile{
				Path:    "commands/bd-plan.md",
				Content: MustReadTemplate("bd-plan.md"),
			},
			InstallFile{
				Path:    "commands/bd-plan-ultra.md",
				Content: MustReadTemplate("bd-plan-ultra.md"),
			},
			InstallFile{
				Path:    "commands/bd-plan-user.md",
				Content: MustReadTemplate("bd-plan-user.md"),
			},
			InstallFile{
				Path:     "CLAUDE.md",
				Content:  MustReadTemplate("claude-md-append.md"),
				IsAppend: true,
			},
		)
	}

	return files
}

// Run executes the init command with the given options.
func Run(opts Options) (*Result, error) {
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}

	// Determine target directory
	targetDir, err := getTargetDir(opts.Global)
	if err != nil {
		return nil, err
	}

	// Build file list
	files := BuildFileList(opts.Minimal)

	// Check file statuses (existence, content changes)
	statuses := checkFileStatuses(targetDir, files)

	if opts.DryRun {
		return showDryRun(opts.Writer, targetDir, files, statuses)
	}

	// Check if any files have changes that require --force
	var changedFiles []FileStatus
	for _, s := range statuses {
		if s.Exists && !s.Unchanged {
			changedFiles = append(changedFiles, s)
		}
	}

	// Handle changed files without --force
	if len(changedFiles) > 0 && !opts.Force {
		return showChanges(opts.Writer, targetDir, statuses)
	}

	// Install files
	return installFiles(opts.Writer, targetDir, files, statuses, opts.Force, opts.Global)
}

// getTargetDir returns the target .claude directory path.
func getTargetDir(global bool) (string, error) {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		return filepath.Join(home, ".claude"), nil
	}
	return ".claude", nil
}

// checkFileStatuses checks each file and returns its status.
func checkFileStatuses(targetDir string, files []InstallFile) []FileStatus {
	var statuses []FileStatus
	for _, f := range files {
		if f.IsAppend {
			continue // Append files don't need status checking
		}

		status := FileStatus{Path: f.Path}
		path := filepath.Join(targetDir, f.Path)

		existingContent, err := os.ReadFile(path)
		if err == nil {
			status.Exists = true
			if string(existingContent) == f.Content {
				status.Unchanged = true
			} else {
				status.Diff = UnifiedDiff("existing", "new", string(existingContent), f.Content)
			}
		}

		statuses = append(statuses, status)
	}
	return statuses
}

// showDryRun displays what would be changed without making changes.
func showDryRun(w io.Writer, targetDir string, files []InstallFile, statuses []FileStatus) (*Result, error) {
	_, _ = fmt.Fprintln(w, "DRY RUN - No changes will be made")
	_, _ = fmt.Fprintln(w)

	result := &Result{TargetDir: targetDir}

	// Build status map for quick lookup
	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Path] = s
	}

	for _, f := range files {
		path := filepath.Join(targetDir, f.Path)

		if f.IsAppend {
			_, _ = fmt.Fprintf(w, "Would append to: %s\n", path)
			_, _ = fmt.Fprintln(w, "--- BEGIN APPEND ---")
			_, _ = fmt.Fprintln(w, f.Content)
			_, _ = fmt.Fprintln(w, "--- END APPEND ---")
			_, _ = fmt.Fprintln(w)
			result.Appended = append(result.Appended, f.Path)
			continue
		}

		status := statusMap[f.Path]
		if status.Exists {
			if status.Unchanged {
				_, _ = fmt.Fprintf(w, "Already up to date: %s\n", path)
				result.Unchanged = append(result.Unchanged, f.Path)
			} else {
				_, _ = fmt.Fprintf(w, "Would overwrite (has changes): %s\n", path)
				_, _ = fmt.Fprintln(w, status.Diff)
				result.Skipped = append(result.Skipped, f.Path)
			}
		} else {
			_, _ = fmt.Fprintf(w, "Would create: %s\n", path)
			_, _ = fmt.Fprintln(w, "--- BEGIN FILE ---")
			_, _ = fmt.Fprintln(w, f.Content)
			_, _ = fmt.Fprintln(w, "--- END FILE ---")
			_, _ = fmt.Fprintln(w)
			result.Created = append(result.Created, f.Path)
		}
	}

	_, _ = fmt.Fprintln(w, "Run without --dry-run to apply changes.")
	return result, nil
}

// showChanges displays files with changes and their diffs.
func showChanges(w io.Writer, targetDir string, statuses []FileStatus) (*Result, error) {
	result := &Result{TargetDir: targetDir}

	// Separate changed and unchanged files
	var changed, unchanged []FileStatus
	for _, s := range statuses {
		if s.Exists {
			if s.Unchanged {
				unchanged = append(unchanged, s)
			} else {
				changed = append(changed, s)
			}
		}
	}

	// Show files with changes and their diffs
	if len(changed) > 0 {
		_, _ = fmt.Fprintln(w, "The following files have changes:")
		_, _ = fmt.Fprintln(w)
		for _, s := range changed {
			path := filepath.Join(targetDir, s.Path)
			_, _ = fmt.Fprintf(w, "%s:\n", path)
			_, _ = fmt.Fprintln(w, s.Diff)
			result.Skipped = append(result.Skipped, s.Path)
		}
	}

	// Show unchanged files
	if len(unchanged) > 0 {
		_, _ = fmt.Fprintln(w, "Already up to date:")
		for _, s := range unchanged {
			_, _ = fmt.Fprintf(w, "  %s\n", filepath.Join(targetDir, s.Path))
			result.Unchanged = append(result.Unchanged, s.Path)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintln(w, "Use --force to overwrite changed files.")
	return result, fmt.Errorf("files have changes (use --force to overwrite)")
}

// installFiles creates directories and writes files.
func installFiles(w io.Writer, targetDir string, files []InstallFile, statuses []FileStatus, force bool, global bool) (*Result, error) {
	result := &Result{TargetDir: targetDir}

	// Build status map for quick lookup
	statusMap := make(map[string]FileStatus)
	for _, s := range statuses {
		statusMap[s.Path] = s
	}

	// Check if all non-append files are unchanged
	allUnchanged := true
	for _, s := range statuses {
		if s.Exists && !s.Unchanged {
			allUnchanged = false
			break
		}
	}

	// If all files are unchanged, just report that
	if allUnchanged && len(statuses) > 0 {
		hasExisting := false
		for _, s := range statuses {
			if s.Exists {
				hasExisting = true
				break
			}
		}
		if hasExisting {
			// Check if there are any new files to create
			hasNew := false
			for _, f := range files {
				if !f.IsAppend {
					if status, ok := statusMap[f.Path]; !ok || !status.Exists {
						hasNew = true
						break
					}
				}
			}
			if !hasNew {
				_, _ = fmt.Fprintln(w, "Already up to date:")
				for _, s := range statuses {
					if s.Exists && s.Unchanged {
						_, _ = fmt.Fprintf(w, "  %s\n", filepath.Join(targetDir, s.Path))
						result.Unchanged = append(result.Unchanged, s.Path)
					}
				}
				_, _ = fmt.Fprintln(w)
				_, _ = fmt.Fprintln(w, "Claude Code configuration is already up to date.")
				return result, nil
			}
		}
	}

	for _, f := range files {
		path := filepath.Join(targetDir, f.Path)

		// Create directory
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return result, fmt.Errorf("create directory %s: %w", dir, err)
		}

		// Handle existing files
		_, statErr := os.Stat(path)
		exists := statErr == nil

		if f.IsAppend {
			file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return result, fmt.Errorf("open %s: %w", path, err)
			}

			// Add separator if file has content
			if exists {
				info, _ := file.Stat()
				if info.Size() > 0 {
					_, _ = file.WriteString("\n\n")
				}
			}

			_, err = file.WriteString(f.Content)
			_ = file.Close()
			if err != nil {
				return result, fmt.Errorf("write %s: %w", path, err)
			}
			_, _ = fmt.Fprintf(w, "Appended: %s\n", path)
			result.Appended = append(result.Appended, f.Path)
			continue
		}

		status := statusMap[f.Path]
		if status.Exists {
			if status.Unchanged {
				_, _ = fmt.Fprintf(w, "Already up to date: %s\n", path)
				result.Unchanged = append(result.Unchanged, f.Path)
				continue
			}
			// File has changes - overwrite if force
			if force {
				if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
					return result, fmt.Errorf("write %s: %w", path, err)
				}
				_, _ = fmt.Fprintf(w, "Overwritten: %s\n", path)
				result.Overwritten = append(result.Overwritten, f.Path)
			} else {
				_, _ = fmt.Fprintf(w, "Skipped (has changes): %s\n", path)
				result.Skipped = append(result.Skipped, f.Path)
			}
		} else {
			// New file
			if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
				return result, fmt.Errorf("write %s: %w", path, err)
			}
			_, _ = fmt.Fprintf(w, "Created: %s\n", path)
			result.Created = append(result.Created, f.Path)
		}
	}

	_, _ = fmt.Fprintln(w)

	// Show git backup recommendation for global installs
	if global {
		_, _ = fmt.Fprintln(w, "Tip: We recommend backing up your ~/.claude directory with git.")
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintln(w, "Claude Code configuration initialized successfully!")
	_, _ = fmt.Fprintln(w, "You can now use 'atari start' to begin processing beads.")

	return result, nil
}
