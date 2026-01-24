package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.viper)
}

func TestConfigInit(t *testing.T) {
	// Create temporary directory for config testing
	tmpDir := t.TempDir()

	// Create a test config file
	configPath := filepath.Join(tmpDir, "govital.yaml")
	configContent := `log_level: debug`
	err := os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Create a new viper instance for this test
	testViper := viper.New()
	testViper.SetConfigName("govital")
	testViper.SetConfigType("yaml")
	testViper.AddConfigPath(tmpDir)

	cfg := &Config{viper: testViper}
	cfg.Init()

	// Verify config was loaded
	logLevel := cfg.viper.GetString("log_level")
	assert.Equal(t, "debug", logLevel)
}

func TestConfigInitDefaults(t *testing.T) {
	cfg := NewConfig()
	cfg.Init()

	// Should have default values even if config file doesn't exist
	logLevel := cfg.viper.GetString("log_level")
	assert.NotEmpty(t, logLevel)
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name           string
		logLevelStr    string
		expectedLevel  slog.Level
	}{
		{
			name:           "debug level",
			logLevelStr:    "debug",
			expectedLevel:  slog.LevelDebug,
		},
		{
			name:           "info level",
			logLevelStr:    "info",
			expectedLevel:  slog.LevelInfo,
		},
		{
			name:           "warn level",
			logLevelStr:    "warn",
			expectedLevel:  slog.LevelWarn,
		},
		{
			name:           "error level",
			logLevelStr:    "error",
			expectedLevel:  slog.LevelError,
		},
		{
			name:           "unknown level defaults to info",
			logLevelStr:    "unknown",
			expectedLevel:  slog.LevelInfo,
		},
		{
			name:           "empty level defaults to info",
			logLevelStr:    "",
			expectedLevel:  slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.viper.Set("log_level", tt.logLevelStr)

			level := cfg.GetLogLevel()

			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

func TestGetLogLevelString(t *testing.T) {
	tests := []struct {
		name          string
		logLevelStr   string
		expectedStr   string
	}{
		{
			name:          "valid debug level",
			logLevelStr:   "debug",
			expectedStr:   "debug",
		},
		{
			name:          "valid info level",
			logLevelStr:   "info",
			expectedStr:   "info",
		},
		{
			name:          "empty level defaults to info",
			logLevelStr:   "",
			expectedStr:   "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.viper.Set("log_level", tt.logLevelStr)

			levelStr := cfg.GetLogLevelString()

			assert.Equal(t, tt.expectedStr, levelStr)
		})
	}
}

func TestConfigViper(t *testing.T) {
	assert.NotNil(t, Viper)
}

func TestGetStaleThresholdDays(t *testing.T) {
	tests := []struct {
		name        string
		configValue int
		expected    int
	}{
		{
			name:        "default 30 days",
			configValue: 30,
			expected:    30,
		},
		{
			name:        "custom 180 days",
			configValue: 180,
			expected:    180,
		},
		{
			name:        "lenient 365 days",
			configValue: 365,
			expected:    365,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.viper.Set("scanner.stale_threshold_days", tt.configValue)

			result := cfg.GetStaleThresholdDays()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetActiveThresholdDays(t *testing.T) {
	tests := []struct {
		name        string
		configValue int
		expected    int
	}{
		{
			name:        "default 90 days",
			configValue: 90,
			expected:    90,
		},
		{
			name:        "custom 180 days",
			configValue: 180,
			expected:    180,
		},
		{
			name:        "lenient 365 days",
			configValue: 365,
			expected:    365,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.viper.Set("scanner.active_threshold_days", tt.configValue)

			result := cfg.GetActiveThresholdDays()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetStaleThresholdDays(t *testing.T) {
	cfg := NewConfig()
	cfg.SetStaleThresholdDays(120)

	result := cfg.GetStaleThresholdDays()

	assert.Equal(t, 120, result)
}

func TestSetActiveThresholdDays(t *testing.T) {
	cfg := NewConfig()
	cfg.SetActiveThresholdDays(60)

	result := cfg.GetActiveThresholdDays()

	assert.Equal(t, 60, result)
}

func TestGetIncludeIndirectDependencies(t *testing.T) {
	tests := []struct {
		name        string
		configValue bool
		expected    bool
	}{
		{
			name:        "default false (only direct)",
			configValue: false,
			expected:    false,
		},
		{
			name:        "true (include indirect)",
			configValue: true,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			cfg.viper.Set("scanner.include_indirect_dependencies", tt.configValue)

			result := cfg.GetIncludeIndirectDependencies()

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetIncludeIndirectDependencies(t *testing.T) {
	cfg := NewConfig()
	cfg.SetIncludeIndirectDependencies(true)

	result := cfg.GetIncludeIndirectDependencies()

	assert.True(t, result)
}
