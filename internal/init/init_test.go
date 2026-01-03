package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildFileList(t *testing.T) {
	t.Run("full install", func(t *testing.T) {
		files := BuildFileList(false)
		if len(files) != 6 {
			t.Errorf("expected 6 files, got %d", len(files))
		}

		// Check expected files are present
		paths := make(map[string]bool)
		for _, f := range files {
			paths[f.Path] = true
		}

		expectedPaths := []string{
			"rules/issue-tracking.md",
			"rules/session-protocol.md",
			"skills/bd-issue-tracking.md",
			"commands/bd-plan.md",
			"commands/bd-plan-ultra.md",
			"CLAUDE.md",
		}
		for _, p := range expectedPaths {
			if !paths[p] {
				t.Errorf("expected file %s not found", p)
			}
		}

		// Check CLAUDE.md is append
		for _, f := range files {
			if f.Path == "CLAUDE.md" && !f.IsAppend {
				t.Error("CLAUDE.md should have IsAppend=true")
			}
		}
	})

	t.Run("minimal install", func(t *testing.T) {
		files := BuildFileList(true)
		if len(files) != 1 {
			t.Errorf("expected 1 file for minimal, got %d", len(files))
		}
		if files[0].Path != "rules/issue-tracking.md" {
			t.Errorf("expected issue-tracking.md, got %s", files[0].Path)
		}
	})
}

func TestMustReadTemplate(t *testing.T) {
	// Test that all templates can be read
	templates := []string{
		"issue-tracking.md",
		"session-protocol.md",
		"bd-issue-tracking.md",
		"bd-plan.md",
		"bd-plan-ultra.md",
		"claude-md-append.md",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			content := MustReadTemplate(tmpl)
			if content == "" {
				t.Errorf("template %s is empty", tmpl)
			}
			if !strings.Contains(content, "#") {
				t.Errorf("template %s should contain markdown headers", tmpl)
			}
		})
	}
}

func TestRun_DryRun(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		DryRun: true,
		Writer: &buf,
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check dry run banner
	if !strings.Contains(output, "DRY RUN") {
		t.Error("expected DRY RUN banner")
	}

	// Check files would be created
	if !strings.Contains(output, "Would create") {
		t.Error("expected 'Would create' in output")
	}

	// Check result has target dir
	if result.TargetDir != ".claude" {
		t.Errorf("expected target dir .claude, got %s", result.TargetDir)
	}

	// Check files are listed in created (since dir doesn't exist)
	if len(result.Created) == 0 {
		t.Error("expected created files in result")
	}
}

func TestRun_Install(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var buf bytes.Buffer
	opts := Options{
		Writer: &buf,
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check files were created
	expectedFiles := []string{
		".claude/rules/issue-tracking.md",
		".claude/rules/session-protocol.md",
		".claude/skills/bd-issue-tracking.md",
		".claude/commands/bd-plan.md",
		".claude/commands/bd-plan-ultra.md",
		".claude/CLAUDE.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created", f)
		}
	}

	// Check result
	if len(result.Created) != 5 {
		t.Errorf("expected 5 created files, got %d", len(result.Created))
	}
	if len(result.Appended) != 1 {
		t.Errorf("expected 1 appended file, got %d", len(result.Appended))
	}

	// Check output messages
	output := buf.String()
	if !strings.Contains(output, "Created:") {
		t.Error("expected 'Created:' in output")
	}
	if !strings.Contains(output, "initialized successfully") {
		t.Error("expected success message")
	}
}

func TestRun_ConflictsWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing file
	if err := os.MkdirAll(".claude/rules", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(".claude/rules/issue-tracking.md", []byte("existing"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Writer: &buf,
	}

	_, err := Run(opts)
	if err == nil {
		t.Fatal("expected error for conflicts without force")
	}

	output := buf.String()
	if !strings.Contains(output, "already exist") {
		t.Error("expected 'already exist' message")
	}
	if !strings.Contains(output, "--force") {
		t.Error("expected --force hint")
	}
}

func TestRun_ForceCreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing file
	if err := os.MkdirAll(".claude/rules", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existingContent := "existing content"
	if err := os.WriteFile(".claude/rules/issue-tracking.md", []byte(existingContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Force:  true,
		Writer: &buf,
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check backup was created
	if len(result.BackedUp) != 1 {
		t.Errorf("expected 1 backup, got %d", len(result.BackedUp))
	}

	// Check backup file exists
	files, _ := filepath.Glob(".claude/rules/issue-tracking*.bak")
	if len(files) != 1 {
		t.Errorf("expected 1 backup file, got %d", len(files))
	}

	// Check backup contains original content
	if len(files) > 0 {
		content, _ := os.ReadFile(files[0])
		if string(content) != existingContent {
			t.Errorf("backup content mismatch: got %q", string(content))
		}
	}

	// Check new file was created
	newContent, _ := os.ReadFile(".claude/rules/issue-tracking.md")
	if string(newContent) == existingContent {
		t.Error("new file should have different content")
	}

	output := buf.String()
	if !strings.Contains(output, "Backed up:") {
		t.Error("expected 'Backed up:' in output")
	}
}

func TestRun_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	var buf bytes.Buffer
	opts := Options{
		Minimal: true,
		Writer:  &buf,
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only issue-tracking.md should be created
	if len(result.Created) != 1 {
		t.Errorf("expected 1 created file for minimal, got %d", len(result.Created))
	}

	// Check that session-protocol.md was NOT created
	if _, err := os.Stat(".claude/rules/session-protocol.md"); !os.IsNotExist(err) {
		t.Error("session-protocol.md should not exist in minimal mode")
	}

	// Check that skills dir was NOT created
	if _, err := os.Stat(".claude/skills"); !os.IsNotExist(err) {
		t.Error("skills directory should not exist in minimal mode")
	}
}

func TestRun_Global(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		DryRun: true,
		Global: true,
		Writer: &buf,
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check target dir is in home directory
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude")
	if result.TargetDir != expected {
		t.Errorf("expected target dir %s, got %s", expected, result.TargetDir)
	}
}

func TestRun_AppendToCLAUDEMd(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing CLAUDE.md
	if err := os.MkdirAll(".claude", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existingContent := "# Existing Config\n\nSome existing content."
	if err := os.WriteFile(".claude/CLAUDE.md", []byte(existingContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Writer: &buf,
	}

	_, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check CLAUDE.md was appended, not replaced
	content, _ := os.ReadFile(".claude/CLAUDE.md")
	contentStr := string(content)

	if !strings.HasPrefix(contentStr, existingContent) {
		t.Error("existing CLAUDE.md content should be preserved")
	}

	if !strings.Contains(contentStr, "BD Integration") {
		t.Error("new content should be appended")
	}
}

func TestBackupPath(t *testing.T) {
	path := backupPath("/some/path/file.md")

	// Should contain timestamp pattern
	if !strings.Contains(path, "file.") {
		t.Error("backup path should contain original filename")
	}
	if !strings.HasSuffix(path, ".md.bak") {
		t.Error("backup path should end with .md.bak")
	}

	// Should contain date pattern
	if !strings.Contains(path, "T") {
		t.Error("backup path should contain timestamp separator")
	}
}
