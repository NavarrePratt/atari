package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/npratt/atari/internal/config"
)

// DaemonInfo contains connection information for the daemon.
// This is written to daemon.json so CLI commands can find the daemon
// regardless of which directory they're run from.
type DaemonInfo struct {
	SocketPath string    `json:"socket_path"`
	PIDPath    string    `json:"pid_path"`
	LogPath    string    `json:"log_path"`
	StartTime  time.Time `json:"start_time"`
	PID        int       `json:"pid"`
}

// daemonInfoFile is the name of the file containing daemon connection info.
const daemonInfoFile = "daemon.json"

// projectMarkers are directories that indicate project root.
var projectMarkers = []string{".git", ".beads"}

// ResolvePaths converts relative paths to absolute paths using the given base directory.
// If basePath is empty, the current working directory is used.
func ResolvePaths(paths config.PathsConfig, basePath string) (config.PathsConfig, error) {
	if basePath == "" {
		var err error
		basePath, err = os.Getwd()
		if err != nil {
			return paths, fmt.Errorf("get working directory: %w", err)
		}
	}

	resolve := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(basePath, p)
	}

	return config.PathsConfig{
		State:  resolve(paths.State),
		Log:    resolve(paths.Log),
		Socket: resolve(paths.Socket),
		PID:    resolve(paths.PID),
	}, nil
}

// FindProjectRoot walks up the directory tree from startDir looking for
// project markers (.git or .beads). Returns the directory containing
// the marker, or startDir if no marker is found.
func FindProjectRoot(startDir string) string {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "."
		}
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return startDir
	}

	dir := absDir
	for {
		// Check for project markers
		for _, marker := range projectMarkers {
			markerPath := filepath.Join(dir, marker)
			if info, err := os.Stat(markerPath); err == nil && info.IsDir() {
				return dir
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding marker
			return absDir
		}
		dir = parent
	}
}

// FindDaemonInfo searches for daemon.json starting from startDir and walking up
// to the project root. Returns the DaemonInfo if found, or an error if not found.
func FindDaemonInfo(startDir string) (*DaemonInfo, error) {
	projectRoot := FindProjectRoot(startDir)

	// Check standard location in .atari directory
	infoPath := filepath.Join(projectRoot, ".atari", daemonInfoFile)
	info, err := ReadDaemonInfo(infoPath)
	if err == nil {
		return info, nil
	}

	return nil, fmt.Errorf("daemon info not found (checked %s)", infoPath)
}

// WriteDaemonInfo writes daemon connection info to the specified path.
func WriteDaemonInfo(path string, info *DaemonInfo) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal daemon info: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write daemon info: %w", err)
	}

	return nil
}

// ReadDaemonInfo reads daemon connection info from the specified path.
func ReadDaemonInfo(path string) (*DaemonInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read daemon info: %w", err)
	}

	var info DaemonInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal daemon info: %w", err)
	}

	return &info, nil
}

// RemoveDaemonInfo removes the daemon.json file.
func RemoveDaemonInfo(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove daemon info: %w", err)
	}
	return nil
}

// DaemonInfoPath returns the path to daemon.json in the .atari directory
// relative to the project root.
func DaemonInfoPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".atari", daemonInfoFile)
}
