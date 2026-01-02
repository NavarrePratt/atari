package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrompt_PromptFile(t *testing.T) {
	// Create temp file with custom prompt
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	content := "Custom prompt from file for {{.BeadID}}"
	if err := os.WriteFile(promptFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	cfg := &Config{
		PromptFile: promptFile,
		Prompt:     "inline prompt",
	}

	got, err := cfg.LoadPrompt()
	if err != nil {
		t.Fatalf("LoadPrompt() error = %v", err)
	}
	if got != content {
		t.Errorf("LoadPrompt() = %q, want %q", got, content)
	}
}

func TestLoadPrompt_PromptFileNotFound(t *testing.T) {
	cfg := &Config{
		PromptFile: "/nonexistent/path/prompt.txt",
	}

	_, err := cfg.LoadPrompt()
	if err == nil {
		t.Error("LoadPrompt() expected error for missing file, got nil")
	}
}

func TestLoadPrompt_InlinePrompt(t *testing.T) {
	inlinePrompt := "This is an inline prompt"
	cfg := &Config{
		Prompt: inlinePrompt,
	}

	got, err := cfg.LoadPrompt()
	if err != nil {
		t.Fatalf("LoadPrompt() error = %v", err)
	}
	if got != inlinePrompt {
		t.Errorf("LoadPrompt() = %q, want %q", got, inlinePrompt)
	}
}

func TestLoadPrompt_DefaultPrompt(t *testing.T) {
	cfg := &Config{}

	got, err := cfg.LoadPrompt()
	if err != nil {
		t.Fatalf("LoadPrompt() error = %v", err)
	}
	if got != DefaultPrompt {
		t.Errorf("LoadPrompt() = %q, want DefaultPrompt", got)
	}
}

func TestLoadPrompt_Priority(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	fileContent := "from file"
	if err := os.WriteFile(promptFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}

	// PromptFile should take priority over Prompt
	cfg := &Config{
		PromptFile: promptFile,
		Prompt:     "from inline",
	}

	got, err := cfg.LoadPrompt()
	if err != nil {
		t.Fatalf("LoadPrompt() error = %v", err)
	}
	if got != fileContent {
		t.Errorf("LoadPrompt() = %q, want %q (PromptFile should take priority)", got, fileContent)
	}
}

func TestExpandPrompt(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     PromptVars
		want     string
	}{
		{
			name:     "all variables",
			template: "Bead {{.BeadID}}: {{.BeadTitle}}\n{{.BeadDescription}}\nLabel: {{.Label}}",
			vars: PromptVars{
				BeadID:          "bd-123",
				BeadTitle:       "Fix bug",
				BeadDescription: "This is the description",
				Label:           "high-priority",
			},
			want: "Bead bd-123: Fix bug\nThis is the description\nLabel: high-priority",
		},
		{
			name:     "no variables",
			template: "Plain prompt without variables",
			vars:     PromptVars{},
			want:     "Plain prompt without variables",
		},
		{
			name:     "empty vars",
			template: "ID: {{.BeadID}}, Title: {{.BeadTitle}}",
			vars:     PromptVars{},
			want:     "ID: , Title: ",
		},
		{
			name:     "repeated variables",
			template: "{{.BeadID}} and again {{.BeadID}}",
			vars: PromptVars{
				BeadID: "bd-456",
			},
			want: "bd-456 and again bd-456",
		},
		{
			name:     "only BeadID",
			template: "Work on {{.BeadID}}",
			vars: PromptVars{
				BeadID: "bd-789",
			},
			want: "Work on bd-789",
		},
		{
			name:     "special characters in values",
			template: "Title: {{.BeadTitle}}",
			vars: PromptVars{
				BeadTitle: "Fix {{.template}} injection",
			},
			want: "Title: Fix {{.template}} injection",
		},
		{
			name:     "multiline description",
			template: "Description:\n{{.BeadDescription}}",
			vars: PromptVars{
				BeadDescription: "Line 1\nLine 2\nLine 3",
			},
			want: "Description:\nLine 1\nLine 2\nLine 3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExpandPrompt(tc.template, tc.vars)
			if got != tc.want {
				t.Errorf("ExpandPrompt() =\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestExpandPrompt_NoInjection(t *testing.T) {
	// Ensure that user-provided values cannot inject new template variables
	template := "Working on {{.BeadID}}"
	vars := PromptVars{
		BeadID: "{{.BeadTitle}}", // Attempt to inject another variable
	}

	got := ExpandPrompt(template, vars)
	// The {{.BeadTitle}} should be treated as literal text, not expanded
	want := "Working on {{.BeadTitle}}"
	if got != want {
		t.Errorf("ExpandPrompt() = %q, want %q (should not expand injected variables)", got, want)
	}
}
