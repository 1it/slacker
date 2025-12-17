package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/logger"
	"github.com/1it/slacker/internal/manifest"
	"github.com/1it/slacker/internal/resource"
)

// Result holds the outcome of applying a single resource
type Result struct {
	ID      string
	Type    string
	Changed bool
	Error   error
}

// Runner orchestrates the application of resources
type Runner struct {
	exec   executor.Executor
	dryRun bool
}

// New creates a new Runner with the given executor
func New(exec executor.Executor) *Runner {
	return &Runner{
		exec:   exec,
		dryRun: false,
	}
}

// WithDryRun enables dry-run mode (check only, no changes)
func (r *Runner) WithDryRun(dryRun bool) *Runner {
	r.dryRun = dryRun
	return r
}

// Apply processes all resources from the manifest in order
func (r *Runner) Apply(ctx context.Context, m *manifest.Manifest) ([]Result, error) {
	results := make([]Result, 0, len(m.Resources))
	notifications := make(map[string]bool) // track pending notifications

	// Apply all resources in order
	for _, res := range m.Resources {
		handler, err := resource.FromManifest(res)
		if err != nil {
			results = append(results, Result{
				ID:    res.Name,
				Type:  res.Type,
				Error: err,
			})
			continue
		}

		logger.Infof("[%s] %s: checking", handler.Type(), handler.ID())

		needsChange, err := handler.NeedsChange(ctx, r.exec)
		if err != nil {
			results = append(results, Result{
				ID:    handler.ID(),
				Type:  handler.Type(),
				Error: fmt.Errorf("check failed: %w", err),
			})
			continue
		}

		if !needsChange {
			logger.Infof("[%s] %s: up to date", handler.Type(), handler.ID())
			results = append(results, Result{
				ID:      handler.ID(),
				Type:    handler.Type(),
				Changed: false,
			})
			continue
		}

		// In dry-run mode, only report what would change
		if r.dryRun {
			logger.Warnf("[%s] %s: would change (dry-run)", handler.Type(), handler.ID())
			results = append(results, Result{
				ID:      handler.ID(),
				Type:    handler.Type(),
				Changed: true, // Mark as "would change"
			})
			if handler.Notifies() != "" {
				notifications[handler.Notifies()] = true
			}
			continue
		}

		logger.Infof("[%s] %s: applying", handler.Type(), handler.ID())

		changed, err := handler.Apply(ctx, r.exec)
		if err != nil {
			results = append(results, Result{
				ID:    handler.ID(),
				Type:  handler.Type(),
				Error: fmt.Errorf("apply failed: %w", err),
			})
			continue
		}

		results = append(results, Result{
			ID:      handler.ID(),
			Type:    handler.Type(),
			Changed: changed,
		})

		// Track notification if resource changed
		if changed && handler.Notifies() != "" {
			notifications[handler.Notifies()] = true
			logger.Debugf("[%s] %s: queued notification %s", handler.Type(), handler.ID(), handler.Notifies())
		}
	}

	// Execute pending notifications (deduplicated)
	if len(notifications) > 0 {
		if r.dryRun {
			logger.Warnf("Would process %d notifications (dry-run):", len(notifications))
			for notify := range notifications {
				logger.Warnf("  - %s", notify)
			}
		} else {
			logger.Infof("Processing %d notifications", len(notifications))
			for notify := range notifications {
				if err := r.executeNotification(ctx, notify); err != nil {
					logger.Errorf(err, "Notification failed: %s", notify)
				}
			}
		}
	}

	return results, nil
}

// executeNotification parses and executes a notification string
// Format: "service:name:action" e.g., "service:nginx:restart"
func (r *Runner) executeNotification(ctx context.Context, notify string) error {
	parts := strings.SplitN(notify, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid notification format %q: expected 'type:name:action'", notify)
	}

	resourceType, name, action := parts[0], parts[1], parts[2]
	logger.Infof("Executing notification: %s %s %s", resourceType, name, action)

	switch resourceType {
	case "service":
		return r.notifyService(ctx, name, action)
	default:
		return fmt.Errorf("unsupported notification type: %s", resourceType)
	}
}

// notifyService handles service notifications (restart, reload, etc.)
func (r *Runner) notifyService(ctx context.Context, name, action string) error {
	var args []string
	switch action {
	case "restart":
		args = []string{"restart", name}
	case "reload":
		args = []string{"reload", name}
	case "start":
		args = []string{"start", name}
	case "stop":
		args = []string{"stop", name}
	default:
		return fmt.Errorf("unsupported service action: %s", action)
	}

	_, stderr, exitCode, err := r.exec.Run(ctx, "systemctl", args...)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("systemctl %s %s failed (exit %d): %s", action, name, exitCode, string(stderr))
	}

	return nil
}

// Summary returns a summary of the run results
func Summary(results []Result) (total, changed, failed int) {
	for _, r := range results {
		total++
		if r.Error != nil {
			failed++
		} else if r.Changed {
			changed++
		}
	}
	return
}
