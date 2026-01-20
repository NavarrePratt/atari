package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory and returns it along with a cleanup function.
// The cleanup function removes the directory and all its contents.
func TempDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "atari-test-*")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

// WriteFile writes content to a file in the given directory.
// It creates parent directories as needed and returns the full path.
func WriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ReadFile reads a file and returns its contents.
// It fails the test if the file cannot be read.
func ReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// FileExists checks if a file exists.
func FileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// AssertCalled verifies that a command was called with the expected args.
func AssertCalled(t *testing.T, mock *MockRunner, name string, args ...string) {
	t.Helper()
	calls := mock.GetCalls()
	for _, call := range calls {
		if call.Name == name && slicesEqual(call.Args, args) {
			return
		}
	}
	t.Errorf("expected call to %s %v not found in %v", name, args, calls)
}

// AssertNotCalled verifies that a command was NOT called.
func AssertNotCalled(t *testing.T, mock *MockRunner, name string) {
	t.Helper()
	calls := mock.GetCalls()
	for _, call := range calls {
		if call.Name == name {
			t.Errorf("unexpected call to %s found: %v", name, call)
			return
		}
	}
}

// AssertCallCount verifies the number of times a command was called.
func AssertCallCount(t *testing.T, mock *MockRunner, name string, expected int) {
	t.Helper()
	count := 0
	calls := mock.GetCalls()
	for _, call := range calls {
		if call.Name == name {
			count++
		}
	}
	if count != expected {
		t.Errorf("expected %d calls to %s, got %d (calls: %v)", expected, name, count, calls)
	}
}

// slicesEqual compares two string slices for equality.
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

// SetupTestDir creates a test directory with common atari structure.
// Returns the directory path and cleanup function.
func SetupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, cleanup := TempDir(t)

	// Create .atari directory
	atariDir := filepath.Join(dir, ".atari")
	if err := os.MkdirAll(atariDir, 0755); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return dir, cleanup
}

// SetupTestDirWithState creates a test directory with state file.
// Returns the directory path and cleanup function.
func SetupTestDirWithState(t *testing.T, stateJSON string) (string, func()) {
	t.Helper()
	dir, cleanup := SetupTestDir(t)

	WriteFile(t, dir, ".atari/state.json", stateJSON)

	return dir, cleanup
}

// SetupMockBRReady configures a MockRunner to respond to br ready commands.
func SetupMockBRReady(mock *MockRunner, response string) {
	mock.SetResponse("br", []string{"ready", "--json"}, []byte(response))
}

// SetupMockBRClose configures a MockRunner to respond to br close commands.
func SetupMockBRClose(mock *MockRunner, beadID string) {
	mock.SetResponse("br", []string{"close", beadID}, []byte(BRCloseSuccess))
}
