package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunWithGracefulShutdown starts a component and handles graceful shutdown.
// The runner function should block while the component is running.
func RunWithGracefulShutdown(
	ctx context.Context,
	logger *slog.Logger,
	timeout time.Duration,
	runner func(ctx context.Context) error,
	shutdown func(ctx context.Context) error,
) error {
	// Create cancellable context for the runner
	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	// Channel to receive runner completion
	runDone := make(chan error, 1)
	go func() {
		runDone <- runner(runCtx)
	}()

	// Wait for signal or runner completion
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("received signal, initiating shutdown", "signal", sig)
		runCancel()

		// Wait for runner to finish with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
		defer shutdownCancel()

		if err := shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "error", err)
		}

		// Wait for runner to complete
		select {
		case err := <-runDone:
			if err != nil && err != context.Canceled {
				return err
			}
		case <-shutdownCtx.Done():
			logger.Warn("shutdown timeout exceeded")
		}

		logger.Info("shutdown complete")
		return nil

	case err := <-runDone:
		return err
	}
}
