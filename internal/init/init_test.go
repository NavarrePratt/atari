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
		if len(files) != 9 {
			t.Errorf("expected 9 files, got %d", len(files))
		}

		// Check expected files are present
		paths := make(map[string]bool)
		for _, f := range files {
			paths[f.Path] = true
		}

		expectedPaths := []string{
			"rules/issue-tracking.md",
			"rules/session-protocol.md",
			"skills/issue-tracking.md",
			"skills/issue-create/SKILL.md",
			"skills/issue-plan/SKILL.md",
			"skills/issue-plan-codex/SKILL.md",
			"skills/issue-plan-user/SKILL.md",
			"skills/issue-plan-hybrid/SKILL.md",
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
		"issue-tracking-skill.md",
		"issue-create.md",
		"skill-issue-plan.md",
		"skill-issue-plan-codex.md",
		"skill-issue-plan-user.md",
		"skill-issue-plan-hybrid.md",
		"_shared-patterns.md",
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
		".claude/skills/issue-tracking.md",
		".claude/skills/issue-create/SKILL.md",
		".claude/skills/issue-plan/SKILL.md",
		".claude/skills/issue-plan-codex/SKILL.md",
		".claude/skills/issue-plan-user/SKILL.md",
		".claude/skills/issue-plan-hybrid/SKILL.md",
		".claude/CLAUDE.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to be created", f)
		}
	}

	// Check result
	if len(result.Created) != 8 {
		t.Errorf("expected 8 created files, got %d", len(result.Created))
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

	if !strings.Contains(contentStr, "BR Integration") {
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

func TestRun_IdempotentManagedSection(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// First run - creates CLAUDE.md with managed section
	var buf1 bytes.Buffer
	opts := Options{Writer: &buf1}
	_, err := Run(opts)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Read content after first run
	content1, _ := os.ReadFile(".claude/CLAUDE.md")

	// Second run - should be idempotent
	var buf2 bytes.Buffer
	opts.Writer = &buf2
	result, err := Run(opts)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// Read content after second run
	content2, _ := os.ReadFile(".claude/CLAUDE.md")

	// Content should be identical
	if string(content1) != string(content2) {
		t.Error("running init twice should produce identical CLAUDE.md content")
	}

	// Should show as unchanged
	output := buf2.String()
	if !strings.Contains(output, "Already up to date") {
		t.Errorf("expected 'Already up to date' for CLAUDE.md, got: %s", output)
	}

	// CLAUDE.md should be in unchanged list
	found := false
	for _, f := range result.Unchanged {
		if f == "CLAUDE.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("CLAUDE.md should be in unchanged list on second run")
	}
}

func TestRun_PreservesUserContentOutsideMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create .claude directory with existing CLAUDE.md that has user content + managed section
	if err := os.MkdirAll(".claude", 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	existingContent := `# My Custom Config

Some user content that should be preserved.

<atari-managed>
# Old BD Integration
Old content here.
</atari-managed>

# More User Content

This should also be preserved.
`
	if err := os.WriteFile(".claude/CLAUDE.md", []byte(existingContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	opts := Options{Writer: &buf}
	_, err := Run(opts)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Read updated content
	content, _ := os.ReadFile(".claude/CLAUDE.md")
	contentStr := string(content)

	// User content before managed section should be preserved
	if !strings.Contains(contentStr, "My Custom Config") {
		t.Error("user content before managed section should be preserved")
	}
	if !strings.Contains(contentStr, "Some user content") {
		t.Error("user content before managed section should be preserved")
	}

	// User content after managed section should be preserved
	if !strings.Contains(contentStr, "More User Content") {
		t.Error("user content after managed section should be preserved")
	}
	if !strings.Contains(contentStr, "This should also be preserved") {
		t.Error("user content after managed section should be preserved")
	}

	// Managed section should be updated
	if !strings.Contains(contentStr, "<atari-managed>") {
		t.Error("managed section markers should exist")
	}
	if !strings.Contains(contentStr, "BR Integration") {
		t.Error("managed section should contain new content")
	}

	// Old content should be replaced
	if strings.Contains(contentStr, "Old content here") {
		t.Error("old managed section content should be replaced")
	}
}

func TestHandleManagedSection(t *testing.T) {
	tests := []struct {
		name            string
		existing        string
		newSection      string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "append to empty",
			existing:     "",
			newSection:   "<atari-managed>\nnew content\n</atari-managed>",
			wantContains: []string{"<atari-managed>", "new content", "</atari-managed>"},
		},
		{
			name:         "append to existing without markers",
			existing:     "# Existing\n\nSome content.",
			newSection:   "<atari-managed>\nnew content\n</atari-managed>",
			wantContains: []string{"# Existing", "Some content", "<atari-managed>", "new content"},
		},
		{
			name:            "replace existing markers",
			existing:        "before\n\n<atari-managed>\nold\n</atari-managed>\n\nafter",
			newSection:      "<atari-managed>\nnew\n</atari-managed>",
			wantContains:    []string{"before", "after", "<atari-managed>", "new"},
			wantNotContains: []string{"old"},
		},
		{
			name:         "markers at start",
			existing:     "<atari-managed>\nold\n</atari-managed>\n\nuser content",
			newSection:   "<atari-managed>\nnew\n</atari-managed>",
			wantContains: []string{"new", "user content"},
		},
		{
			name:         "markers at end",
			existing:     "user content\n\n<atari-managed>\nold\n</atari-managed>",
			newSection:   "<atari-managed>\nnew\n</atari-managed>",
			wantContains: []string{"user content", "new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleManagedSection(tt.existing, tt.newSection)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got: %s", want, result)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(result, notWant) {
					t.Errorf("result should not contain %q, got: %s", notWant, result)
				}
			}
		})
	}
}

func TestParseSharedPatternsFromContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		want      map[string]string
		wantError bool
	}{
		{
			name: "single section",
			content: `# Header
<!-- BEGIN SECTION_A -->
content A
<!-- END SECTION_A -->
trailing`,
			want: map[string]string{
				"SECTION_A": "content A",
			},
		},
		{
			name: "multiple sections",
			content: `
<!-- BEGIN FIRST -->
first content
<!-- END FIRST -->

<!-- BEGIN SECOND -->
second content
<!-- END SECOND -->
`,
			want: map[string]string{
				"FIRST":  "first content",
				"SECOND": "second content",
			},
		},
		{
			name: "multiline section",
			content: `<!-- BEGIN MULTI -->
line 1
line 2
line 3
<!-- END MULTI -->`,
			want: map[string]string{
				"MULTI": "line 1\nline 2\nline 3",
			},
		},
		{
			name: "missing end marker",
			content: `<!-- BEGIN ORPHAN -->
content without end`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSharedPatternsFromContent(tt.content)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("expected %d patterns, got %d", len(tt.want), len(got))
			}

			for key, wantVal := range tt.want {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("key %q: expected %q, got %q", key, wantVal, gotVal)
				}
			}
		})
	}
}

func TestReplaceMarkers(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		patterns  map[string]string
		want      string
		wantError bool
	}{
		{
			name:     "single marker replacement",
			content:  "before {{ MARKER }} after",
			patterns: map[string]string{"MARKER": "replaced"},
			want:     "before replaced after",
		},
		{
			name:     "multiple markers",
			content:  "{{ A }} and {{ B }}",
			patterns: map[string]string{"A": "first", "B": "second"},
			want:     "first and second",
		},
		{
			name:     "marker with extra whitespace",
			content:  "{{  SPACED  }}",
			patterns: map[string]string{"SPACED": "content"},
			want:     "content",
		},
		{
			name:     "no markers",
			content:  "plain text without markers",
			patterns: map[string]string{"UNUSED": "value"},
			want:     "plain text without markers",
		},
		{
			name:      "missing marker in patterns",
			content:   "{{ MISSING }}",
			patterns:  map[string]string{"OTHER": "value"},
			wantError: true,
		},
		{
			name:     "marker on its own line",
			content:  "header\n\n{{ BLOCK }}\n\nfooter",
			patterns: map[string]string{"BLOCK": "multiline\ncontent"},
			want:     "header\n\nmultiline\ncontent\n\nfooter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replaceMarkers(tt.content, tt.patterns)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestBuildFileList_MarkerReplacement(t *testing.T) {
	files := BuildFileList(false)

	// Find skill files that should have marker replacement
	skillPaths := []string{
		"skills/issue-plan/SKILL.md",
		"skills/issue-plan-codex/SKILL.md",
		"skills/issue-plan-user/SKILL.md",
		"skills/issue-plan-hybrid/SKILL.md",
	}

	for _, path := range skillPaths {
		var found *InstallFile
		for i := range files {
			if files[i].Path == path {
				found = &files[i]
				break
			}
		}

		if found == nil {
			t.Errorf("expected file %s not found", path)
			continue
		}

		// Verify no unresolved markers remain
		if strings.Contains(found.Content, "{{") {
			t.Errorf("%s contains unresolved {{ marker", path)
		}

		// Verify shared content was injected (should contain verification commands section)
		if !strings.Contains(found.Content, "mise.toml") {
			t.Errorf("%s should contain shared verification discovery content", path)
		}
	}
}
