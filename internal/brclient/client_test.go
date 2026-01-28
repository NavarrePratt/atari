package brclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/npratt/atari/internal/testutil"
)

func TestCLIClient_Show(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		response  []byte
		wantErr   bool
		wantBead  *Bead
		errSubstr string
	}{
		{
			name: "success",
			id:   "bd-001",
			response: []byte(`[{
				"id": "bd-001",
				"title": "Test bead",
				"status": "open",
				"priority": 1
			}]`),
			wantBead: &Bead{
				ID:       "bd-001",
				Title:    "Test bead",
				Status:   "open",
				Priority: 1,
			},
		},
		{
			name:      "empty response",
			id:        "bd-999",
			response:  []byte{},
			wantErr:   true,
			errSubstr: "bead not found",
		},
		{
			name:      "empty array",
			id:        "bd-999",
			response:  []byte(`[]`),
			wantErr:   true,
			errSubstr: "bead not found",
		},
		{
			name:      "invalid json",
			id:        "bd-001",
			response:  []byte(`not json`),
			wantErr:   true,
			errSubstr: "parse br show output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.SetResponse("br", []string{"show", tt.id, "--json"}, tt.response)

			client := NewCLIClient(runner)
			bead, err := client.Show(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if bead.ID != tt.wantBead.ID {
				t.Errorf("ID = %q, want %q", bead.ID, tt.wantBead.ID)
			}
			if bead.Title != tt.wantBead.Title {
				t.Errorf("Title = %q, want %q", bead.Title, tt.wantBead.Title)
			}
		})
	}
}

func TestCLIClient_List(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ListOptions
		response []byte
		wantLen  int
		wantArgs []string
	}{
		{
			name:     "no options",
			opts:     nil,
			response: []byte(`[{"id": "bd-001"}, {"id": "bd-002"}]`),
			wantLen:  2,
			wantArgs: []string{"list", "--json"},
		},
		{
			name:     "with status filter",
			opts:     &ListOptions{Status: "closed"},
			response: []byte(`[{"id": "bd-001"}]`),
			wantLen:  1,
			wantArgs: []string{"list", "--json", "--status", "closed"},
		},
		{
			name:     "empty response",
			opts:     nil,
			response: []byte{},
			wantLen:  0,
			wantArgs: []string{"list", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.SetResponse("br", tt.wantArgs, tt.response)

			client := NewCLIClient(runner)
			beads, err := client.List(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(beads) != tt.wantLen {
				t.Errorf("got %d beads, want %d", len(beads), tt.wantLen)
			}

			calls := runner.GetCalls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}
			if !slicesEqual(calls[0].Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", calls[0].Args, tt.wantArgs)
			}
		})
	}
}

func TestCLIClient_Ready(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ReadyOptions
		response []byte
		wantLen  int
		wantArgs []string
	}{
		{
			name:     "no options",
			opts:     nil,
			response: []byte(`[{"id": "bd-001"}]`),
			wantLen:  1,
			wantArgs: []string{"ready", "--json"},
		},
		{
			name:     "with label",
			opts:     &ReadyOptions{Label: "atari"},
			response: []byte(`[{"id": "bd-001"}]`),
			wantLen:  1,
			wantArgs: []string{"ready", "--json", "--label", "atari"},
		},
		{
			name:     "unassigned only",
			opts:     &ReadyOptions{UnassignedOnly: true},
			response: []byte(`[]`),
			wantLen:  0,
			wantArgs: []string{"ready", "--json", "--unassigned"},
		},
		{
			name:     "both options",
			opts:     &ReadyOptions{Label: "atari", UnassignedOnly: true},
			response: []byte(`[{"id": "bd-001"}]`),
			wantLen:  1,
			wantArgs: []string{"ready", "--json", "--label", "atari", "--unassigned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.SetResponse("br", tt.wantArgs, tt.response)

			client := NewCLIClient(runner)
			beads, err := client.Ready(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(beads) != tt.wantLen {
				t.Errorf("got %d beads, want %d", len(beads), tt.wantLen)
			}

			calls := runner.GetCalls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}
			if !slicesEqual(calls[0].Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", calls[0].Args, tt.wantArgs)
			}
		})
	}
}

func TestCLIClient_Labels(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetResponse("br", []string{"label", "list", "bd-001", "--json"}, []byte(`["label1", "label2"]`))

	client := NewCLIClient(runner)
	labels, err := client.Labels(context.Background(), "bd-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(labels) != 2 {
		t.Errorf("got %d labels, want 2", len(labels))
	}
	if labels[0] != "label1" || labels[1] != "label2" {
		t.Errorf("labels = %v, want [label1, label2]", labels)
	}
}

func TestCLIClient_UpdateStatus(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		status   string
		notes    string
		wantArgs []string
	}{
		{
			name:     "status only",
			id:       "bd-001",
			status:   "open",
			notes:    "",
			wantArgs: []string{"update", "bd-001", "--status", "open"},
		},
		{
			name:     "with notes",
			id:       "bd-001",
			status:   "open",
			notes:    "Reset for retry",
			wantArgs: []string{"update", "bd-001", "--status", "open", "--notes", "Reset for retry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.SetResponse("br", tt.wantArgs, []byte(`{}`))

			client := NewCLIClient(runner)
			err := client.UpdateStatus(context.Background(), tt.id, tt.status, tt.notes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			calls := runner.GetCalls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}
			if !slicesEqual(calls[0].Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", calls[0].Args, tt.wantArgs)
			}
		})
	}
}

func TestCLIClient_Close(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetResponse("br", []string{"close", "bd-001", "--reason", "Completed successfully"}, []byte(`{}`))

	client := NewCLIClient(runner)
	err := client.Close(context.Background(), "bd-001", "Completed successfully")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := runner.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	expected := []string{"close", "bd-001", "--reason", "Completed successfully"}
	if !slicesEqual(calls[0].Args, expected) {
		t.Errorf("args = %v, want %v", calls[0].Args, expected)
	}
}

func TestCLIClient_CloseEligibleEpics(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		wantLen  int
	}{
		{
			name:     "epics closed",
			response: []byte(`[{"id": "bd-epic-001", "title": "Epic 1", "dependent_count": 3}]`),
			wantLen:  1,
		},
		{
			name:     "no epics closed",
			response: []byte{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			runner.SetResponse("br", []string{"epic", "close-eligible", "--json"}, tt.response)

			client := NewCLIClient(runner)
			results, err := client.CloseEligibleEpics(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(results) != tt.wantLen {
				t.Errorf("got %d results, want %d", len(results), tt.wantLen)
			}
		})
	}
}

func TestCLIClient_CommandError(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetError("br", []string{"show", "bd-001", "--json"}, errors.New("command failed"))

	client := NewCLIClient(runner)
	_, err := client.Show(context.Background(), "bd-001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !contains(err.Error(), "br show bd-001 failed") {
		t.Errorf("error %q does not mention failed command", err.Error())
	}
}

func TestCLIClient_WithTimeout(t *testing.T) {
	runner := testutil.NewMockRunner()
	client := NewCLIClient(runner)

	if client.timeout != DefaultTimeout {
		t.Errorf("default timeout = %v, want %v", client.timeout, DefaultTimeout)
	}

	newClient := client.WithTimeout(5 * time.Second)
	if newClient.timeout != 5*time.Second {
		t.Errorf("new timeout = %v, want 5s", newClient.timeout)
	}
	if client.timeout != DefaultTimeout {
		t.Error("original client timeout was modified")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
