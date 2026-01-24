package cmd

import (
	"github.com/spf13/cobra"
	"github.com/steffakasid/eslog"
	"github.com/steffakasid/govital/pkg/config"
	"github.com/steffakasid/govital/pkg/scanner"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan Go project dependencies for maintenance status",
	Long: `Scan all dependencies of a Go project and check if they are 
actively maintained and if the used versions are up to date.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := cmd.Flags().GetString("project-path")
		if err != nil {
			return err
		}

		staleThreshold, err := cmd.Flags().GetInt("stale-threshold")
		if err != nil {
			return err
		}

		includeIndirect, err := cmd.Flags().GetBool("include-indirect")
		if err != nil {
			return err
		}

		workers, err := cmd.Flags().GetInt("workers")
		if err != nil {
			return err
		}

		eslog.Infof("Starting dependency scan: %s", projectPath)

		s := scanner.NewScanner(projectPath)

		// Use CLI flag if provided, otherwise use config
		if cmd.Flags().Changed("stale-threshold") {
			s.SetStaleThreshold(staleThreshold)
		} else {
			cfg := config.NewConfig()
			s.SetStaleThreshold(cfg.GetStaleThresholdDays())
		}

		if cmd.Flags().Changed("include-indirect") {
			s.SetIncludeIndirectDependencies(includeIndirect)
		} else {
			cfg := config.NewConfig()
			s.SetIncludeIndirectDependencies(cfg.GetIncludeIndirectDependencies())
		}

		if cmd.Flags().Changed("workers") {
			s.SetWorkers(workers)
		}

		if err := s.Scan(); err != nil {
			eslog.Errorf("Scan failed: %v", err)
			return err
		}

		s.PrintResults()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringP("project-path", "p", ".", "Path to the Go project to scan")
	scanCmd.Flags().IntP("stale-threshold", "t", 30, "Number of days a dependency can be inactive before marked as stale")
	scanCmd.Flags().BoolP("include-indirect", "i", false, "Include indirect (transitive) dependencies in the scan")
	scanCmd.Flags().IntP("workers", "w", 4, "Number of parallel workers for scanning dependencies")
}
