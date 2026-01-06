package tui

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// jsonlEntry represents a single bead entry from the JSONL file.
type jsonlEntry struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	Priority     int        `json:"priority"`
	IssueType    string     `json:"issue_type"`
	CreatedAt    string     `json:"created_at"`
	CreatedBy    string     `json:"created_by"`
	UpdatedAt    string     `json:"updated_at"`
	ClosedAt     string     `json:"closed_at,omitempty"`
	CloseReason  string     `json:"close_reason,omitempty"`
	Parent       string     `json:"parent,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	DeletedAt    string     `json:"deleted_at,omitempty"`
	Dependencies []jsonlDep `json:"dependencies,omitempty"`
}

// jsonlDep represents a dependency entry in the JSONL file.
type jsonlDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"` // "blocks" or "parent-child"
}

// jsonlBeadInfo holds basic info about a bead for building references.
type jsonlBeadInfo struct {
	title  string
	status string
}

// depRef holds a dependency reference for the reverse index.
type depRef struct {
	id      string
	depType string
}

// JSONLReader reads bead data directly from the .beads/issues.jsonl file.
type JSONLReader struct {
	beadsDir string
}

// NewJSONLReader creates a new JSONLReader for the given beads directory.
func NewJSONLReader(beadsDir string) *JSONLReader {
	return &JSONLReader{beadsDir: beadsDir}
}

// ReadAll reads all beads from the JSONL file and returns them with full dependency data.
// Deleted entries are filtered out.
func (r *JSONLReader) ReadAll() ([]GraphBead, error) {
	entries, err := r.readAndParse()
	if err != nil {
		return nil, err
	}

	return r.convertEntries(entries), nil
}

// readAndParse reads the JSONL file and parses all valid entries.
// Malformed lines are logged and skipped. Deleted entries are filtered out.
func (r *JSONLReader) readAndParse() ([]jsonlEntry, error) {
	filePath := filepath.Join(r.beadsDir, "issues.jsonl")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	entries := make([]jsonlEntry, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry jsonlEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed trailing line silently (partial write)
			if i == len(lines)-1 {
				continue
			}
			// Log warning for malformed middle lines
			slog.Warn("skipping malformed JSONL line",
				"line", i+1,
				"error", err.Error())
			continue
		}

		// Filter out deleted entries
		if entry.DeletedAt != "" {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// convertEntries converts jsonlEntry slice to GraphBead slice with resolved dependencies.
// Uses two-pass algorithm: first builds indices, then converts with resolved references.
func (r *JSONLReader) convertEntries(entries []jsonlEntry) []GraphBead {
	// Pass 1: Build bead index and dependents index
	beadIndex := make(map[string]jsonlBeadInfo, len(entries))
	dependentsIndex := make(map[string][]depRef)

	for _, entry := range entries {
		beadIndex[entry.ID] = jsonlBeadInfo{
			title:  entry.Title,
			status: entry.Status,
		}

		// Build reverse index: for each dependency, add this entry as a dependent
		for _, dep := range entry.Dependencies {
			// dep.IssueID is the bead that has this dependency
			// dep.DependsOnID is the bead being depended on
			// So DependsOnID has IssueID as a dependent
			dependentsIndex[dep.DependsOnID] = append(dependentsIndex[dep.DependsOnID], depRef{
				id:      dep.IssueID,
				depType: dep.Type,
			})
		}
	}

	// Pass 2: Convert entries to GraphBead with resolved references
	beads := make([]GraphBead, 0, len(entries))

	for _, entry := range entries {
		bead := GraphBead{
			ID:              entry.ID,
			Title:           entry.Title,
			Description:     entry.Description,
			Status:          entry.Status,
			Priority:        entry.Priority,
			IssueType:       entry.IssueType,
			CreatedAt:       entry.CreatedAt,
			CreatedBy:       entry.CreatedBy,
			UpdatedAt:       entry.UpdatedAt,
			ClosedAt:        entry.ClosedAt,
			Parent:          entry.Parent,
			Notes:           entry.Notes,
			DependencyCount: len(entry.Dependencies),
		}

		// Resolve dependencies
		bead.Dependencies = r.convertDeps(entry.Dependencies, beadIndex)

		// Resolve dependents from reverse index
		if deps, ok := dependentsIndex[entry.ID]; ok {
			bead.DependentCount = len(deps)
			bead.Dependents = make([]BeadReference, 0, len(deps))
			for _, dep := range deps {
				info := beadIndex[dep.id]
				bead.Dependents = append(bead.Dependents, BeadReference{
					ID:             dep.id,
					Title:          info.title,
					Status:         info.status,
					DependencyType: dep.depType,
				})
			}
		}

		beads = append(beads, bead)
	}

	return beads
}

// convertDeps converts jsonlDep slice to BeadReference slice with resolved info.
func (r *JSONLReader) convertDeps(deps []jsonlDep, beadIndex map[string]jsonlBeadInfo) []BeadReference {
	if len(deps) == 0 {
		return nil
	}

	refs := make([]BeadReference, 0, len(deps))
	for _, dep := range deps {
		info := beadIndex[dep.DependsOnID]
		refs = append(refs, BeadReference{
			ID:             dep.DependsOnID,
			Title:          info.title,
			Status:         info.status,
			DependencyType: dep.Type,
		})
	}

	return refs
}
