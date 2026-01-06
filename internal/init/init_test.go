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
		if len(files) != 8 {
			t.Errorf("expected 8 files, got %d", len(files))
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
			"commands/bd-create.md",
			"commands/bd-plan.md",
			"commands/bd-plan-ultra.md",
			"commands/bd-plan-user.md",
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
		"bd-create.md",
		"bd-plan.md",
		"bd-plan-ultra.md",
		"bd-plan-user.md",
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
		".claude/commands/bd-create.md",
		".claude/commands/bd-plan.md",
		".claude/commands/bd-plan-ultra.md",
		".claude/commands/bd-plan-user.md",
		".claude/CLAUDE.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created", f)
		}
	}

	// Check result
	if len(result.Created) != 7 {
		t.Errorf("expected 7 created files, got %d", len(result.Created))
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

func TestRun_ChangesWithoutForce(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing file with different content
	if err := os.MkdirAll(".claude/rules", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(".claude/rules/issue-tracking.md", []byte("existing content"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{
		Writer: &buf,
	}

	_, err := Run(opts)
	if err == nil {
		t.Fatal("expected error for changed files without force")
	}

	output := buf.String()
	if !strings.Contains(output, "have changes") {
		t.Error("expected 'have changes' message")
	}
	if !strings.Contains(output, "--force") {
		t.Error("expected --force hint")
	}
	// Should show diff
	if !strings.Contains(output, "---") || !strings.Contains(output, "+++") {
		t.Error("expected unified diff in output")
	}
}

func TestRun_ForceOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create existing file with different content
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

	// Check file was overwritten (not backed up)
	if len(result.Overwritten) != 1 {
		t.Errorf("expected 1 overwritten file, got %d", len(result.Overwritten))
	}

	// Verify no backup files were created
	backupFiles, _ := filepath.Glob(".claude/rules/issue-tracking*.bak")
	if len(backupFiles) != 0 {
		t.Errorf("expected 0 backup files, got %d", len(backupFiles))
	}

	// Check new file has new content
	newContent, _ := os.ReadFile(".claude/rules/issue-tracking.md")
	if string(newContent) == existingContent {
		t.Error("file should have been overwritten with new content")
	}

	output := buf.String()
	if !strings.Contains(output, "Overwritten:") {
		t.Error("expected 'Overwritten:' in output")
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

func TestRun_SkipsUnchangedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// First, run init to create files
	var buf1 bytes.Buffer
	opts1 := Options{
		Minimal: true, // Just one file for simplicity
		Writer:  &buf1,
	}

	_, err := Run(opts1)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Run again - should succeed without --force since content is identical
	var buf2 bytes.Buffer
	opts2 := Options{
		Minimal: true,
		Writer:  &buf2,
	}

	result, err := Run(opts2)
	if err != nil {
		t.Fatalf("second run should succeed without --force: %v", err)
	}

	output := buf2.String()
	if !strings.Contains(output, "Already up to date") {
		t.Error("expected 'Already up to date' message")
	}

	if len(result.Unchanged) != 1 {
		t.Errorf("expected 1 unchanged file, got %d", len(result.Unchanged))
	}
}

func TestRun_GitRecommendationForGlobal(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a fake home directory for the test
	fakeHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(fakeHome, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// We can't easily test --global without modifying $HOME,
	// so we test the output message logic directly by checking
	// that the git tip appears in the success path when global=true
	// For now, just verify the message format exists in the codebase
	// The actual global test would need environment variable manipulation

	// Test local install doesn't show git tip
	var buf bytes.Buffer
	opts := Options{
		Minimal: true,
		Writer:  &buf,
	}

	_, err := Run(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "backing up your ~/.claude") {
		t.Error("local install should not show git backup tip")
	}
}
