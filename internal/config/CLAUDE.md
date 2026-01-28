# Config Package

Type definitions and defaults for atari configuration.

## Structure

```go
Config
├── Claude        // Session settings
│   ├── Timeout   // Per-session timeout (default: 60m)
│   └── ExtraArgs // Additional CLI args
├── WorkQueue     // Polling settings
│   ├── PollInterval // bd ready poll interval (default: 5s)
│   └── Label        // Label filter for work selection
├── Backoff       // Failed bead retry settings
│   ├── Initial     // First retry delay (default: 1m)
│   ├── Max         // Maximum delay (default: 1h)
│   ├── Multiplier  // Exponential factor (default: 2.0)
│   └── MaxFailures // Abandon threshold (default: 5)
├── Paths         // File locations
│   ├── State   // .atari/state.json
│   ├── Log     // .atari/atari.log
│   ├── Socket  // .atari/atari.sock
│   └── PID     // .atari/atari.pid
├── BDActivity    // BD activity watcher settings
│   └── Enabled   // Enable watcher (default: true)
└── Prompt        // Default session prompt
```

## Usage

```go
// Get defaults
cfg := config.Default()

// Override as needed
cfg.Claude.Timeout = 10 * time.Minute
cfg.WorkQueue.Label = "priority:high"
```

## DefaultPrompt

The `DefaultPrompt` constant instructs Claude Code to:
1. Run `bd ready --json` for work discovery
2. Use available skills and MCPs
3. Implement highest-priority issue completely
4. Create new bd issues for discovered bugs

## Extending

To add a new config section:
1. Define a new struct type (e.g., `NewFeatureConfig`)
2. Add field to `Config` struct
3. Set defaults in `Default()` function
4. Document in `docs/config/configuration.md`
