/*
Copyright © 2025 1it <0x1it@pm.me>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/1it/slacker/internal/manifest"
	"github.com/spf13/cobra"
)

var configFile string

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy configuration to target hosts",
	Long: `Deploy applies the configuration manifest to all specified hosts.

The manifest file defines hosts and actions (packages, files, services)
to configure on each target server.

Example:
  slacker deploy -c manifest.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if configFile == "" {
			return fmt.Errorf("config file is required: use -c or --config")
		}

		m, err := manifest.Load(configFile)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Loaded manifest with %d hosts and %d actions\n",
			len(m.Hosts), len(m.Resources))

		for i, host := range m.Hosts {
			fmt.Fprintf(os.Stdout, "  Host %d: %s (user: %s)\n", i+1, host.Address, host.User)
		}

		for i, resource := range m.Resources {
			fmt.Fprintf(os.Stdout, "  Resource %d: [%s] %s\n", i+1, resource.Type, resource.Name)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVarP(&configFile, "config", "c", "", "path to manifest YAML file (required)")
	deployCmd.MarkFlagRequired("config")
}
