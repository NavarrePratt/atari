package exec

import (
	"context"
	"errors"
	"testing"
)

type mockExecCmd struct {
	output []byte
	err    error
}

func (m mockExecCmd) Output() ([]byte, error) {
	return m.output, m.err
}

func TestExecRunner_Run(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
		mockErr    error
		wantErr    bool
	}{
		{
			name:       "successful command",
			mockOutput: []byte("hello world"),
			mockErr:    nil,
			wantErr:    false,
		},
		{
			name:       "command error",
			mockOutput: nil,
			mockErr:    errors.New("command failed"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace execCommand with mock
			origExecCommand := execCommand
			defer func() { execCommand = origExecCommand }()

			execCommand = func(ctx context.Context, name string, args ...string) execCmd {
				return mockExecCmd{output: tt.mockOutput, err: tt.mockErr}
			}

			runner := NewExecRunner()
			output, err := runner.Run(context.Background(), "test", "arg1", "arg2")

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && string(output) != string(tt.mockOutput) {
				t.Errorf("Run() output = %q, want %q", output, tt.mockOutput)
			}
		})
	}
}

func TestNewExecRunner(t *testing.T) {
	runner := NewExecRunner()
	if runner == nil {
		t.Error("NewExecRunner() returned nil")
	}
}
