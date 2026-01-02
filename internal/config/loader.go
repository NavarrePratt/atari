package config

import (
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// ConfigPaths defines the search locations for config files.
const (
	// GlobalConfigDir is the XDG config directory name
	GlobalConfigDir = "atari"
	// GlobalConfigFile is the global config file name
	GlobalConfigFile = "config.yaml"
	// ProjectConfigDir is the project-local config directory
	ProjectConfigDir = ".atari"
	// ProjectConfigFile is the project-local config file name
	ProjectConfigFile = "config.yaml"
)

// LoadConfig loads configuration from files and viper settings.
// Precedence (later overrides earlier):
//  1. Default() values
//  2. ~/.config/atari/config.yaml (global)
//  3. .atari/config.yaml (project)
//  4. Environment variables (ATARI_*)
//  5. CLI flags (already bound to viper)
//
// Missing config files are silently ignored.
func LoadConfig(v *viper.Viper) (*Config, error) {
	// Start with defaults
	cfg := Default()

	// Marshal defaults to map for viper
	defaultMap, err := structToMap(cfg)
	if err != nil {
		return nil, err
	}
	if err := v.MergeConfigMap(defaultMap); err != nil {
		return nil, err
	}

	// Load global config (~/.config/atari/config.yaml)
	globalPath := globalConfigPath()
	if globalPath != "" {
		if err := loadConfigFile(v, globalPath); err != nil {
			return nil, err
		}
	}

	// Load project config (.atari/config.yaml)
	projectPath := projectConfigPath()
	if projectPath != "" {
		if err := loadConfigFile(v, projectPath); err != nil {
			return nil, err
		}
	}

	// Explicit config file (from --config flag or ATARI_CONFIG env)
	if explicitPath := v.GetString("config"); explicitPath != "" {
		// Explicit config must exist
		if _, err := os.Stat(explicitPath); err != nil {
			return nil, err
		}
		if err := loadConfigFile(v, explicitPath); err != nil {
			return nil, err
		}
	}

	// Unmarshal with duration hook
	if err := v.Unmarshal(cfg, viperDecodeHook()); err != nil {
		return nil, err
	}

	return cfg, nil
}

// globalConfigPath returns the global config file path if it exists.
func globalConfigPath() string {
	// Try XDG_CONFIG_HOME first
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		// Fall back to ~/.config
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configDir = filepath.Join(home, ".config")
	}

	path := filepath.Join(configDir, GlobalConfigDir, GlobalConfigFile)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// projectConfigPath returns the project config file path if it exists.
func projectConfigPath() string {
	path := filepath.Join(ProjectConfigDir, ProjectConfigFile)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// loadConfigFile loads a YAML config file and merges it into viper.
// Returns nil if the file doesn't exist.
func loadConfigFile(v *viper.Viper, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Create a temporary viper to read the file
	fileViper := viper.New()
	fileViper.SetConfigType("yaml")
	if err := fileViper.ReadConfig(file); err != nil {
		return err
	}

	// Merge into main viper
	return v.MergeConfigMap(fileViper.AllSettings())
}

// viperDecodeHook returns the decoder config with duration hook.
func viperDecodeHook() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))
}

// structToMap converts a struct to a map for viper.MergeConfigMap.
func structToMap(cfg *Config) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "mapstructure",
		Result:  &result,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			durationToStringHook(),
		),
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	return result, nil
}

// durationToStringHook converts time.Duration to string for YAML compatibility.
func durationToStringHook() mapstructure.DecodeHookFunc {
	return func(from, to reflect.Type, data interface{}) (interface{}, error) {
		if from != reflect.TypeOf(time.Duration(0)) {
			return data, nil
		}
		return data.(time.Duration).String(), nil
	}
}
