package brclient

import (
	"context"
	"sync"
)

// DynamicShowFunc is a callback for dynamic Show responses.
type DynamicShowFunc func(ctx context.Context, id string) (*Bead, error, bool)

// DynamicReadyFunc is a callback for dynamic Ready responses.
type DynamicReadyFunc func(ctx context.Context, opts *ReadyOptions) ([]Bead, error, bool)

// MockClient is a mock implementation of Client for testing.
// It records all calls and returns configured responses.
type MockClient struct {
	mu sync.Mutex

	// Configured responses
	ShowResponses       map[string]*Bead
	ShowErrors          map[string]error
	ListResponse        []Bead
	ListError           error
	LabelsResponses     map[string][]string
	LabelsErrors        map[string]error
	ReadyResponse       []Bead
	ReadyError          error
	UpdateStatusError   error
	UpdateNotesError    error
	CommentError        error
	CloseError          error
	CloseEligibleResult []EpicCloseResult
	CloseEligibleError  error

	// Dynamic response callbacks
	DynamicShow  DynamicShowFunc
	DynamicReady DynamicReadyFunc

	// Call tracking
	ShowCalls          []string
	ListCalls          []*ListOptions
	LabelsCalls        []string
	ReadyCalls         []*ReadyOptions
	UpdateStatusCalls  []UpdateStatusCall
	UpdateNotesCalls   []UpdateNotesCall
	CommentCalls       []CommentCall
	CloseCalls         []CloseCall
	CloseEligibleCalls int
}

// UpdateStatusCall records an UpdateStatus call.
type UpdateStatusCall struct {
	ID     string
	Status string
	Notes  string
}

// UpdateNotesCall records an UpdateNotes call.
type UpdateNotesCall struct {
	ID    string
	Notes string
}

// CommentCall records a Comment call.
type CommentCall struct {
	ID      string
	Message string
}

// CloseCall records a Close call.
type CloseCall struct {
	ID     string
	Reason string
}

// NewMockClient creates a new MockClient with initialized maps.
func NewMockClient() *MockClient {
	return &MockClient{
		ShowResponses:   make(map[string]*Bead),
		ShowErrors:      make(map[string]error),
		LabelsResponses: make(map[string][]string),
		LabelsErrors:    make(map[string]error),
	}
}

// Show implements BeadReader.
func (m *MockClient) Show(ctx context.Context, id string) (*Bead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShowCalls = append(m.ShowCalls, id)

	// Check dynamic callback first
	if m.DynamicShow != nil {
		if bead, err, handled := m.DynamicShow(ctx, id); handled {
			return bead, err
		}
	}

	if err, ok := m.ShowErrors[id]; ok {
		return nil, err
	}
	if bead, ok := m.ShowResponses[id]; ok {
		return bead, nil
	}
	return nil, nil
}

// List implements BeadReader and WorkQueueClient.
func (m *MockClient) List(ctx context.Context, opts *ListOptions) ([]Bead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ListCalls = append(m.ListCalls, opts)

	if m.ListError != nil {
		return nil, m.ListError
	}
	return m.ListResponse, nil
}

// Labels implements BeadReader.
func (m *MockClient) Labels(ctx context.Context, id string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LabelsCalls = append(m.LabelsCalls, id)

	if err, ok := m.LabelsErrors[id]; ok {
		return nil, err
	}
	if labels, ok := m.LabelsResponses[id]; ok {
		return labels, nil
	}
	return nil, nil
}

// Ready implements WorkQueueClient.
func (m *MockClient) Ready(ctx context.Context, opts *ReadyOptions) ([]Bead, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ReadyCalls = append(m.ReadyCalls, opts)

	// Check dynamic callback first
	if m.DynamicReady != nil {
		if beads, err, handled := m.DynamicReady(ctx, opts); handled {
			return beads, err
		}
	}

	if m.ReadyError != nil {
		return nil, m.ReadyError
	}
	return m.ReadyResponse, nil
}

// UpdateStatus implements BeadUpdater.
func (m *MockClient) UpdateStatus(ctx context.Context, id, status, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateStatusCalls = append(m.UpdateStatusCalls, UpdateStatusCall{
		ID:     id,
		Status: status,
		Notes:  notes,
	})

	return m.UpdateStatusError
}

// UpdateNotes implements BeadUpdater.
func (m *MockClient) UpdateNotes(ctx context.Context, id, notes string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateNotesCalls = append(m.UpdateNotesCalls, UpdateNotesCall{
		ID:    id,
		Notes: notes,
	})

	return m.UpdateNotesError
}

// Comment implements BeadUpdater.
func (m *MockClient) Comment(ctx context.Context, id, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CommentCalls = append(m.CommentCalls, CommentCall{
		ID:      id,
		Message: message,
	})

	return m.CommentError
}

// Close implements BeadUpdater.
func (m *MockClient) Close(ctx context.Context, id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCalls = append(m.CloseCalls, CloseCall{
		ID:     id,
		Reason: reason,
	})

	return m.CloseError
}

// CloseEligibleEpics implements BeadUpdater.
func (m *MockClient) CloseEligibleEpics(ctx context.Context) ([]EpicCloseResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseEligibleCalls++

	if m.CloseEligibleError != nil {
		return nil, m.CloseEligibleError
	}
	return m.CloseEligibleResult, nil
}

// SetShowResponse configures a Show response for a bead ID.
func (m *MockClient) SetShowResponse(id string, bead *Bead) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShowResponses[id] = bead
}

// SetShowError configures a Show error for a bead ID.
func (m *MockClient) SetShowError(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShowErrors[id] = err
}

// Reset clears all recorded calls.
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShowCalls = nil
	m.ListCalls = nil
	m.LabelsCalls = nil
	m.ReadyCalls = nil
	m.UpdateStatusCalls = nil
	m.UpdateNotesCalls = nil
	m.CommentCalls = nil
	m.CloseCalls = nil
	m.CloseEligibleCalls = 0
}

// Verify MockClient implements all interfaces.
var (
	_ Client          = (*MockClient)(nil)
	_ BeadReader      = (*MockClient)(nil)
	_ BeadUpdater     = (*MockClient)(nil)
	_ WorkQueueClient = (*MockClient)(nil)
)
