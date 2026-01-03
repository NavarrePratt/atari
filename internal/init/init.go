package initcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	TargetDir string
	Created   []string
	Appended  []string
	Skipped   []string
	BackedUp  []string
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

	// Check for conflicts
	conflicts := checkConflicts(targetDir, files)

	if opts.DryRun {
		return showDryRun(opts.Writer, targetDir, files, conflicts)
	}

	// Handle conflicts without --force
	if len(conflicts) > 0 && !opts.Force {
		return showConflicts(opts.Writer, targetDir, conflicts)
	}

	// Install files
	return installFiles(opts.Writer, targetDir, files, opts.Force)
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

// checkConflicts returns a list of files that already exist.
func checkConflicts(targetDir string, files []InstallFile) []string {
	var conflicts []string
	for _, f := range files {
		if f.IsAppend {
			continue // Append files don't conflict
		}
		path := filepath.Join(targetDir, f.Path)
		if _, err := os.Stat(path); err == nil {
			conflicts = append(conflicts, f.Path)
		}
	}
	return conflicts
}

// showDryRun displays what would be changed without making changes.
func showDryRun(w io.Writer, targetDir string, files []InstallFile, conflicts []string) (*Result, error) {
	_, _ = fmt.Fprintln(w, "DRY RUN - No changes will be made")
	_, _ = fmt.Fprintln(w)

	result := &Result{TargetDir: targetDir}

	for _, f := range files {
		path := filepath.Join(targetDir, f.Path)

		// Check if file exists
		exists := false
		for _, c := range conflicts {
			if c == f.Path {
				exists = true
				break
			}
		}

		if f.IsAppend {
			_, _ = fmt.Fprintf(w, "Would append to: %s\n", path)
			_, _ = fmt.Fprintln(w, "--- BEGIN APPEND ---")
			_, _ = fmt.Fprintln(w, f.Content)
			_, _ = fmt.Fprintln(w, "--- END APPEND ---")
			_, _ = fmt.Fprintln(w)
			result.Appended = append(result.Appended, f.Path)
		} else if exists {
			_, _ = fmt.Fprintf(w, "Would skip (exists): %s\n", path)
			_, _ = fmt.Fprintln(w)
			result.Skipped = append(result.Skipped, f.Path)
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

// showConflicts displays existing files and returns an error.
func showConflicts(w io.Writer, targetDir string, conflicts []string) (*Result, error) {
	_, _ = fmt.Fprintln(w, "The following files already exist:")
	for _, c := range conflicts {
		_, _ = fmt.Fprintf(w, "  %s\n", filepath.Join(targetDir, c))
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Use --force to backup and replace existing files.")
	return &Result{TargetDir: targetDir, Skipped: conflicts}, fmt.Errorf("files already exist (use --force to overwrite)")
}

// installFiles creates directories and writes files.
func installFiles(w io.Writer, targetDir string, files []InstallFile, force bool) (*Result, error) {
	result := &Result{TargetDir: targetDir}

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

		if exists && !f.IsAppend {
			if force {
				// Backup existing file with timestamp
				backup := backupPath(path)
				if err := os.Rename(path, backup); err != nil {
					return result, fmt.Errorf("backup %s: %w", path, err)
				}
				_, _ = fmt.Fprintf(w, "Backed up: %s -> %s\n", path, backup)
				result.BackedUp = append(result.BackedUp, f.Path)
			} else {
				_, _ = fmt.Fprintf(w, "Skipped (exists): %s\n", path)
				result.Skipped = append(result.Skipped, f.Path)
				continue
			}
		}

		// Write or append
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
		} else {
			if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
				return result, fmt.Errorf("write %s: %w", path, err)
			}
			_, _ = fmt.Fprintf(w, "Created: %s\n", path)
			result.Created = append(result.Created, f.Path)
		}
	}

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Claude Code configuration initialized successfully!")
	_, _ = fmt.Fprintln(w, "You can now use 'atari start' to begin processing beads.")

	return result, nil
}

// backupPath returns a timestamped backup path for the given file.
func backupPath(path string) string {
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return fmt.Sprintf("%s.%s%s.bak", base, timestamp, ext)
}
