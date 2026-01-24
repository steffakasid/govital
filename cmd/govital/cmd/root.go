package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/steffakasid/eslog"
	"github.com/steffakasid/govital/pkg/config"
)

var rootCmd = &cobra.Command{
	Use:   "govital",
	Short: "A tool to check if Go dependencies are actively maintained",
	Long: `govital scans all dependencies of a given Go project and checks if those 
dependencies are actively maintained and if the used versions are up to date.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		eslog.Errorf("Failed to execute root command: %v", err)
		os.Exit(1)
	}
}

func init() {
	cfg := config.NewConfig()
	cobra.OnInitialize(func() {
		cfg.Init()
		logLevel := cfg.GetLogLevelString()
		if err := eslog.Logger.SetLogLevel(logLevel); err != nil {
			eslog.Warnf("Failed to set log level: %v", err)
		}
	})

	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Set log level (debug, info, warn, error)")
	_ = config.Viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
}
