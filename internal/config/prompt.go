package config

import (
	"fmt"
	"os"
	"strings"
)

// PromptVars holds variables for prompt template expansion.
type PromptVars struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	Label           string
	BeadParent      string // Parent epic/task ID if any
}

// LoadPrompt returns the prompt template string based on configuration priority:
// PromptFile (load from file) > Prompt (inline) > DefaultPrompt.
// Returns an error if PromptFile is set but the file cannot be read.
func (c *Config) LoadPrompt() (string, error) {
	if c.PromptFile != "" {
		content, err := os.ReadFile(c.PromptFile)
		if err != nil {
			return "", fmt.Errorf("load prompt file %q: %w", c.PromptFile, err)
		}
		return string(content), nil
	}

	if c.Prompt != "" {
		return c.Prompt, nil
	}

	return DefaultPrompt, nil
}

// ExpandPrompt performs variable substitution on a prompt template.
// Uses single-pass replacement to avoid template injection risks.
// Supported variables: {{.BeadID}}, {{.BeadTitle}}, {{.BeadDescription}}, {{.Label}}, {{.BeadParent}}
func ExpandPrompt(template string, vars PromptVars) string {
	// Use Replacer for single-pass replacement to prevent injection
	// (e.g., if BeadID contains "{{.BeadTitle}}", it won't be expanded)
	r := strings.NewReplacer(
		"{{.BeadID}}", vars.BeadID,
		"{{.BeadTitle}}", vars.BeadTitle,
		"{{.BeadDescription}}", vars.BeadDescription,
		"{{.Label}}", vars.Label,
		"{{.BeadParent}}", vars.BeadParent,
	)
	return r.Replace(template)
}
