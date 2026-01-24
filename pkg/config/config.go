package config

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"github.com/steffakasid/eslog"
)

var Viper *viper.Viper

type Config struct {
	viper *viper.Viper
}

func init() {
	Viper = viper.New()
}

func NewConfig() *Config {
	return &Config{
		viper: Viper,
	}
}

func (c *Config) Init() {
	c.viper.SetConfigName("govital")
	c.viper.SetConfigType("yaml")
	c.viper.AddConfigPath(".")
	c.viper.AddConfigPath("/etc/govital/")
	c.viper.AddConfigPath(os.ExpandEnv("$HOME/.govital"))

	// Set defaults
	c.viper.SetDefault("log_level", "info")
	c.viper.SetDefault("scanner.stale_threshold_days", 30)
	c.viper.SetDefault("scanner.active_threshold_days", 90)

	// Read config file
	if err := c.viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			eslog.Debugf("Error reading config file: %v", err)
		}
	}
}

func (c *Config) GetLogLevel() slog.Level {
	levelStr := c.viper.GetString("log_level")
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c *Config) GetLogLevelString() string {
	levelStr := c.viper.GetString("log_level")
	if levelStr == "" {
		return "info"
	}
	return levelStr
}

// Scanner configuration

// GetStaleThresholdDays returns the number of days a dependency can be inactive before being marked as stale.
// Default: 365 days
func (c *Config) GetStaleThresholdDays() int {
	return c.viper.GetInt("scanner.stale_threshold_days")
}

// GetActiveThresholdDays returns the number of days a dependency must have been updated within to be considered active.
// Default: 90 days
func (c *Config) GetActiveThresholdDays() int {
	return c.viper.GetInt("scanner.active_threshold_days")
}

// SetStaleThresholdDays sets the stale threshold in the config.
func (c *Config) SetStaleThresholdDays(days int) {
	c.viper.Set("scanner.stale_threshold_days", days)
}

// SetActiveThresholdDays sets the active threshold in the config.
func (c *Config) SetActiveThresholdDays(days int) {
	c.viper.Set("scanner.active_threshold_days", days)
}
