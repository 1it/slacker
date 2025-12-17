/*
Copyright © 2025 1it <0x1it@pm.me>
*/
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/logger"
	"github.com/1it/slacker/internal/manifest"
	"github.com/1it/slacker/internal/runner"
	"github.com/spf13/cobra"
)

var (
	remoteConfigFile string
	remoteDryRun     bool
	remoteTimeout    time.Duration
	skipVerify       bool
)

// remoteCmd represents the remote command
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Apply configuration to remote hosts via SSH",
	Long: `Remote applies the configuration manifest to remote hosts defined in the manifest.

Each host is configured sequentially using SSH connections.

Example:
  slacker remote -c manifest.yaml
  slacker remote -c manifest.yaml --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if remoteConfigFile == "" {
			return fmt.Errorf("config file is required: use -c or --config")
		}

		m, err := manifest.Load(remoteConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		logger.Infof("Loaded manifest: %s (%d hosts, %d resources)", remoteConfigFile, len(m.Hosts), len(m.Resources))

		if len(m.Hosts) == 0 {
			return fmt.Errorf("no hosts found in manifest")
		}

		if len(m.Resources) == 0 {
			return fmt.Errorf("no resources found in manifest")
		}

		if remoteDryRun {
			logger.Warnf("Dry-run mode enabled - no changes will be made")
		}

		ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
		defer cancel()

		// Track overall results
		var totalHosts, successHosts, failedHosts int
		hostResults := make(map[string]error)
		successfulHosts := make([]manifest.Host, 0)

		// Apply to each host sequentially
		for _, host := range m.Hosts {
			totalHosts++

			// Determine auth method for logging
			authMethod := "password"
			if host.Password == "" {
				if host.Key != "" {
					authMethod = fmt.Sprintf("key:%s", host.Key)
				} else {
					// Auto-detect
					keys := executor.DetectSSHKeys()
					if len(keys) > 0 {
						authMethod = fmt.Sprintf("auto-key:%s", keys[0])
					} else {
						authMethod = "no-auth"
					}
				}
			}
			logger.Infof("=== Connecting to %s (user: %s, auth: %s) ===", host.Address, host.User, authMethod)

			// Create SSH executor
			sshExec, err := executor.NewSSHExecutor(host.Address, host.User, host.Password, host.Key)
			if err != nil {
				logger.Errorf(err, "Failed to connect to %s", host.Address)
				hostResults[host.Address] = err
				failedHosts++
				continue
			}

			// Create runner with SSH executor
			r := runner.New(sshExec).WithDryRun(remoteDryRun)

			// Apply resources
			results, err := r.Apply(ctx, m)

			// Close connection
			if closeErr := sshExec.Close(); closeErr != nil {
				logger.Warnf("Error closing connection to %s: %v", host.Address, closeErr)
			}

			if err != nil {
				logger.Errorf(err, "Apply failed on %s", host.Address)
				hostResults[host.Address] = err
				failedHosts++
				continue
			}

			// Print summary for this host
			total, changed, failed := runner.Summary(results)
			if remoteDryRun {
				logger.Infof("[%s] Dry-run complete: total=%d would_change=%d failed=%d", host.Address, total, changed, failed)
			} else {
				logger.Infof("[%s] Complete: total=%d changed=%d failed=%d", host.Address, total, changed, failed)
			}

			if failed > 0 {
				hostResults[host.Address] = fmt.Errorf("%d resources failed", failed)
				failedHosts++
			} else {
				hostResults[host.Address] = nil
				successHosts++
				successfulHosts = append(successfulHosts, host)
			}
		}

		// Print overall summary
		logger.Infof("=== Summary ===")
		logger.Infof("Hosts: total=%d success=%d failed=%d", totalHosts, successHosts, failedHosts)

		for addr, err := range hostResults {
			if err != nil {
				logger.Errorf(err, "  %s: FAILED", addr)
			} else {
				logger.Infof("  %s: OK", addr)
			}
		}

		if failedHosts > 0 {
			return fmt.Errorf("%d of %d hosts failed", failedHosts, totalHosts)
		}

		// Run verification checks (only if not dry-run and not skipped)
		if len(m.Verify) > 0 && !remoteDryRun && !skipVerify {
			logger.Infof("=== Running Verification Checks ===")
			verifyFailed := runVerifications(ctx, m.Verify, successfulHosts)
			if verifyFailed > 0 {
				return fmt.Errorf("%d verification checks failed", verifyFailed)
			}
		} else if len(m.Verify) > 0 && remoteDryRun {
			logger.Warnf("Skipping %d verification checks (dry-run)", len(m.Verify))
		}

		return nil
	},
}

// runVerifications executes verification commands for each successful host
func runVerifications(ctx context.Context, verifications []manifest.Verify, hosts []manifest.Host) int {
	failed := 0

	for _, v := range verifications {
		for _, host := range hosts {
			// Extract host IP/hostname (remove port)
			hostAddr := host.Address
			if idx := strings.LastIndex(hostAddr, ":"); idx != -1 {
				hostAddr = hostAddr[:idx]
			}

			// Replace ${HOST} placeholder
			command := strings.ReplaceAll(v.Command, "${HOST}", hostAddr)

			logger.Infof("[verify] %s @ %s: %s", v.Name, hostAddr, command)

			// Run command locally
			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			output := stdout.String()

			if err != nil {
				logger.Errorf(err, "[verify] %s @ %s: FAILED - %s", v.Name, hostAddr, stderr.String())
				failed++
				continue
			}

			// Check expected output if specified
			if v.Expect != "" {
				if !strings.Contains(output, v.Expect) {
					logger.Errorf(nil, "[verify] %s @ %s: FAILED - expected %q not found in output", v.Name, hostAddr, v.Expect)
					logger.Infof("[verify] Got: %s", strings.TrimSpace(output))
					failed++
					continue
				}
			}

			logger.Infof("[verify] %s @ %s: OK", v.Name, hostAddr)
			if output != "" {
				logger.Debugf("[verify] Output: %s", strings.TrimSpace(output))
			}
		}
	}

	return failed
}

func init() {
	rootCmd.AddCommand(remoteCmd)
	remoteCmd.Flags().StringVarP(&remoteConfigFile, "config", "c", "", "path to manifest YAML file (required)")
	remoteCmd.Flags().BoolVar(&remoteDryRun, "dry-run", false, "show what would be changed without making changes")
	remoteCmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "skip verification checks after deployment")
	remoteCmd.Flags().DurationVar(&remoteTimeout, "timeout", 30*time.Minute, "timeout for the entire operation")
	_ = remoteCmd.MarkFlagRequired("config")
}
