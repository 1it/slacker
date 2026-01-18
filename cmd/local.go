/*
Copyright © 2025 1it <0x1it@pm.me>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/logger"
	"github.com/1it/slacker/internal/manifest"
	"github.com/1it/slacker/internal/runner"
	"github.com/spf13/cobra"
)

var (
	localConfigFile string
	dryRun          bool
	timeout         time.Duration
)

// localCmd represents the local command
var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Apply configuration to the local machine",
	Long: `Local applies the configuration manifest to the local machine.

This is useful for testing resources before deploying to remote hosts.

Example:
  slacker local -c manifest.yaml
  slacker local -c manifest.yaml --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if localConfigFile == "" {
			return fmt.Errorf("config file is required: use -c or --config")
		}

		m, err := manifest.Load(localConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		// Validate manifest
		if err := m.Validate(); err != nil {
			return err
		}

		logger.Infof("Loaded manifest: %s (%d resources), timeout: %s", localConfigFile, len(m.Resources), timeout)

		if len(m.Resources) == 0 {
			return fmt.Errorf("no resources found in manifest")
		}

		// Create local executor
		exec := &executor.LocalExecutor{}

		// Create runner
		r := runner.New(exec).WithDryRun(dryRun)

		if dryRun {
			logger.Warnf("Dry-run mode enabled - no changes will be made")
		}

		// Apply resources
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		results, err := r.Apply(ctx, m)
		if err != nil {
			return fmt.Errorf("apply failed: %w", err)
		}

		// Print summary
		total, changed, failed := runner.Summary(results)
		if dryRun {
			logger.Warnf("Dry-run complete: total=%d would_change=%d failed=%d", total, changed, failed)
		} else {
			logger.Infof("Run complete: total=%d changed=%d failed=%d", total, changed, failed)
		}

		if failed > 0 {
			return fmt.Errorf("%d resources failed", failed)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(localCmd)
	localCmd.Flags().StringVarP(&localConfigFile, "config", "c", "", "path to manifest YAML file (required)")
	localCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be changed without making changes")
	localCmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "timeout for the operation")
	_ = localCmd.MarkFlagRequired("config")
}
