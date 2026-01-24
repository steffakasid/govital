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
