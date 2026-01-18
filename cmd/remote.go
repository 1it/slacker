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
	"sync"
	"time"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/logger"
	"github.com/1it/slacker/internal/manifest"
	"github.com/1it/slacker/internal/runner"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	remoteConfigFile string
	remoteDryRun     bool
	remoteTimeout    time.Duration
	skipVerify       bool
	parallelHosts    int
	failFast         bool
	strictHostKeys   bool
	acceptNewKeys    bool
	knownHostsFile   string
)

// remoteCmd represents the remote command
var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Apply configuration to remote hosts via SSH",
	Long: `Remote applies the configuration manifest to remote hosts defined in the manifest.

By default, hosts are configured sequentially. Use --parallel to process multiple hosts concurrently.

Example:
  slacker remote -c manifest.yaml
  slacker remote -c manifest.yaml --dry-run
  slacker remote -c manifest.yaml --parallel 5
  slacker remote -c manifest.yaml --parallel 10 --fail-fast`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if remoteConfigFile == "" {
			return fmt.Errorf("config file is required: use -c or --config")
		}

		m, err := manifest.Load(remoteConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		// Validate manifest
		if err := m.Validate(); err != nil {
			return err
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

		// Validate parallel flag
		if parallelHosts < 1 {
			parallelHosts = 1
		}
		if parallelHosts > len(m.Hosts) {
			parallelHosts = len(m.Hosts)
		}

		// Thread-safe result collection
		type hostResult struct {
			host    manifest.Host
			err     error
			total   int
			changed int
			failed  int
		}

		var (
			mu              sync.Mutex
			hostResults     = make(map[string]error)
			successfulHosts = make([]manifest.Host, 0)
			resultsChan     = make(chan hostResult, len(m.Hosts))
		)

		// Build SSH options
		sshOpts := executor.SSHOptions{
			KnownHostsFile: knownHostsFile,
		}
		if strictHostKeys {
			sshOpts.HostKeyMode = executor.HostKeyStrict
			logger.Infof("Host key verification: strict (hosts must be in known_hosts)")
		} else if acceptNewKeys {
			sshOpts.HostKeyMode = executor.HostKeyAcceptNew
			logger.Infof("Host key verification: accept-new (new hosts added to known_hosts)")
		}

		// Log execution mode
		if parallelHosts > 1 {
			logger.Infof("Processing %d hosts with parallelism=%d (fail-fast=%v)", len(m.Hosts), parallelHosts, failFast)
		}

		// Process hosts
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(parallelHosts)

		for _, host := range m.Hosts {
			host := host // capture for goroutine
			g.Go(func() error {
				result := applyToHost(gctx, host, m, remoteDryRun, sshOpts)
				resultsChan <- result

				// In fail-fast mode, return error to cancel other goroutines
				if failFast && result.err != nil {
					return result.err
				}
				return nil
			})
		}

		// Wait for all goroutines and close results channel
		go func() {
			_ = g.Wait()
			close(resultsChan)
		}()

		// Collect results
		for result := range resultsChan {
			mu.Lock()
			hostResults[result.host.Address] = result.err
			if result.err == nil {
				successfulHosts = append(successfulHosts, result.host)
			}
			mu.Unlock()
		}

		// Wait for errgroup to fully complete
		if err := g.Wait(); err != nil && failFast {
			logger.Warnf("Execution stopped due to fail-fast: %v", err)
		}

		// Calculate totals
		totalHosts := len(m.Hosts)
		successHosts := len(successfulHosts)
		failedHosts := totalHosts - successHosts

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

// applyToHost applies the manifest to a single host and returns the result
func applyToHost(ctx context.Context, host manifest.Host, m *manifest.Manifest, dryRun bool, sshOpts executor.SSHOptions) struct {
	host    manifest.Host
	err     error
	total   int
	changed int
	failed  int
} {
	result := struct {
		host    manifest.Host
		err     error
		total   int
		changed int
		failed  int
	}{host: host}

	// Determine auth method for logging
	authMethod := "password"
	if host.Password == "" {
		if host.Key != "" {
			authMethod = fmt.Sprintf("key:%s", host.Key)
		} else {
			keys := executor.DetectSSHKeys()
			if len(keys) > 0 {
				authMethod = fmt.Sprintf("auto-key:%s", keys[0])
			} else {
				authMethod = "no-auth"
			}
		}
	}
	logger.Infof("=== Connecting to %s (user: %s, auth: %s) ===", host.Address, host.User, authMethod)

	// Create SSH executor with options
	sshExec, err := executor.NewSSHExecutorWithOptions(host.Address, host.User, host.Password, host.Key, sshOpts)
	if err != nil {
		logger.Errorf(err, "Failed to connect to %s", host.Address)
		result.err = err
		return result
	}

	// Create runner with SSH executor
	r := runner.New(sshExec).WithDryRun(dryRun)

	// Apply resources
	results, err := r.Apply(ctx, m)

	// Close connection
	if closeErr := sshExec.Close(); closeErr != nil {
		logger.Warnf("Error closing connection to %s: %v", host.Address, closeErr)
	}

	if err != nil {
		logger.Errorf(err, "Apply failed on %s", host.Address)
		result.err = err
		return result
	}

	// Calculate summary
	result.total, result.changed, result.failed = runner.Summary(results)

	// Log result
	if dryRun {
		logger.Infof("[%s] Dry-run complete: total=%d would_change=%d failed=%d", host.Address, result.total, result.changed, result.failed)
	} else {
		logger.Infof("[%s] Complete: total=%d changed=%d failed=%d", host.Address, result.total, result.changed, result.failed)
	}

	if result.failed > 0 {
		result.err = fmt.Errorf("%d resources failed", result.failed)
	}

	return result
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
	remoteCmd.Flags().IntVarP(&parallelHosts, "parallel", "p", 1, "number of hosts to process in parallel (default: 1 = sequential)")
	remoteCmd.Flags().BoolVar(&failFast, "fail-fast", false, "stop processing on first host failure (only with --parallel > 1)")
	remoteCmd.Flags().BoolVar(&strictHostKeys, "strict-host-keys", false, "require hosts to be in known_hosts file")
	remoteCmd.Flags().BoolVar(&acceptNewKeys, "accept-new-keys", false, "accept new host keys and add to known_hosts (reject changed keys)")
	remoteCmd.Flags().StringVar(&knownHostsFile, "known-hosts", "", "path to known_hosts file (default: ~/.ssh/known_hosts)")
	_ = remoteCmd.MarkFlagRequired("config")
}
