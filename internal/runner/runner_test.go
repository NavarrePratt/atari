package runner

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestExecProcessRunner_StartAndWait(t *testing.T) {
	r := NewExecProcessRunner()

	stdout, stderr, err := r.Start(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Read stdout
	out, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("ReadAll stdout failed: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("stdout = %q, want %q", string(out), "hello\n")
	}

	// Stderr should be empty
	errOut, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("ReadAll stderr failed: %v", err)
	}
	if len(errOut) != 0 {
		t.Errorf("stderr = %q, want empty", string(errOut))
	}

	// Wait should succeed
	if err := r.Wait(); err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

func TestExecProcessRunner_Stderr(t *testing.T) {
	r := NewExecProcessRunner()

	// sh -c writes to stderr
	stdout, stderr, err := r.Start(context.Background(), "sh", "-c", "echo error >&2")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Read both streams
	_, _ = io.ReadAll(stdout)
	errOut, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("ReadAll stderr failed: %v", err)
	}
	if string(errOut) != "error\n" {
		t.Errorf("stderr = %q, want %q", string(errOut), "error\n")
	}

	_ = r.Wait()
}

func TestExecProcessRunner_AlreadyStarted(t *testing.T) {
	r := NewExecProcessRunner()

	_, _, err := r.Start(context.Background(), "echo", "first")
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Second start should fail
	_, _, err = r.Start(context.Background(), "echo", "second")
	if err == nil {
		t.Error("Second Start should fail")
	}

	_ = r.Wait()
}

func TestExecProcessRunner_WaitNotStarted(t *testing.T) {
	r := NewExecProcessRunner()

	err := r.Wait()
	if err == nil {
		t.Error("Wait without Start should fail")
	}
}

func TestExecProcessRunner_Kill(t *testing.T) {
	r := NewExecProcessRunner()

	// Start a long-running process
	stdout, _, err := r.Start(context.Background(), "sleep", "10")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Kill should succeed
	if err := r.Kill(); err != nil {
		t.Errorf("Kill failed: %v", err)
	}

	// Wait should return an error (killed)
	err = r.Wait()
	if err == nil {
		t.Error("Wait after Kill should return error")
	}

	_ = stdout.Close()
}

func TestExecProcessRunner_KillNotStarted(t *testing.T) {
	r := NewExecProcessRunner()

	// Kill before start should be safe
	if err := r.Kill(); err != nil {
		t.Errorf("Kill before Start should not error: %v", err)
	}
}

func TestExecProcessRunner_ContextCancel(t *testing.T) {
	r := NewExecProcessRunner()

	ctx, cancel := context.WithCancel(context.Background())

	stdout, _, err := r.Start(ctx, "sleep", "10")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context should terminate process
	cancel()

	// Wait should return context error
	err = r.Wait()
	if err == nil {
		t.Error("Wait after context cancel should return error")
	}

	_ = stdout.Close()
}

func TestExecProcessRunner_ConcurrentAccess(t *testing.T) {
	r := NewExecProcessRunner()

	stdout, stderr, err := r.Start(context.Background(), "sleep", "1")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent Kill calls
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Kill()
		}()
	}

	wg.Wait()
	_ = r.Wait()
	_ = stdout.Close()
	_ = stderr.Close()
}

func TestExecProcessRunner_ExitCode(t *testing.T) {
	r := NewExecProcessRunner()

	stdout, stderr, err := r.Start(context.Background(), "sh", "-c", "exit 42")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Drain the pipes
	_, _ = io.ReadAll(stdout)
	_, _ = io.ReadAll(stderr)

	err = r.Wait()
	if err == nil {
		t.Error("Wait should return error for non-zero exit")
	}
}

func TestExecProcessRunner_StreamingOutput(t *testing.T) {
	r := NewExecProcessRunner()

	// Process that outputs incrementally
	stdout, _, err := r.Start(context.Background(), "sh", "-c", "echo line1; sleep 0.1; echo line2")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Read first line
	buf := make([]byte, 6)
	n, err := io.ReadFull(stdout, buf)
	if err != nil {
		t.Fatalf("Read first line failed: %v", err)
	}
	if string(buf[:n]) != "line1\n" {
		t.Errorf("first read = %q, want %q", string(buf[:n]), "line1\n")
	}

	// Read second line
	n, err = io.ReadFull(stdout, buf)
	if err != nil {
		t.Fatalf("Read second line failed: %v", err)
	}
	if string(buf[:n]) != "line2\n" {
		t.Errorf("second read = %q, want %q", string(buf[:n]), "line2\n")
	}

	_ = r.Wait()
}

func TestExecProcessRunner_InvalidCommand(t *testing.T) {
	r := NewExecProcessRunner()

	_, _, err := r.Start(context.Background(), "nonexistent-command-12345")
	if err == nil {
		t.Error("Start with invalid command should fail")
	}
}

func TestExecProcessRunner_Timeout(t *testing.T) {
	r := NewExecProcessRunner()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stdout, _, err := r.Start(ctx, "sleep", "10")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = r.Wait()
	if err == nil {
		t.Error("Wait should fail after timeout")
	}

	_ = stdout.Close()
}
