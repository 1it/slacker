package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// ServiceHandler manages service resources via systemctl
type ServiceHandler struct {
	name     string
	state    string // "running" or "stopped"
	enabled  bool
	notifies string
}

// NewServiceHandler creates a ServiceHandler from manifest resource
func NewServiceHandler(r manifest.Resource) *ServiceHandler {
	state := r.State
	if state == "" {
		state = "running"
	}
	return &ServiceHandler{
		name:     r.Name,
		state:    state,
		enabled:  r.Enabled,
		notifies: r.Notifies,
	}
}

func (s *ServiceHandler) ID() string       { return fmt.Sprintf("service:%s", s.name) }
func (s *ServiceHandler) Type() string     { return "service" }
func (s *ServiceHandler) Notifies() string { return s.notifies }

// NeedsChange checks if service state differs from desired
func (s *ServiceHandler) NeedsChange(ctx context.Context, exec executor.Executor) (bool, error) {
	running, err := s.isRunning(ctx, exec)
	if err != nil {
		return false, err
	}

	enabled, err := s.isEnabled(ctx, exec)
	if err != nil {
		return false, err
	}

	// Check running state
	if s.state == "running" && !running {
		return true, nil
	}
	if s.state == "stopped" && running {
		return true, nil
	}

	// Check enabled state
	if s.enabled && !enabled {
		return true, nil
	}
	if !s.enabled && enabled {
		return true, nil
	}

	return false, nil
}

// Apply starts/stops and enables/disables the service
func (s *ServiceHandler) Apply(ctx context.Context, exec executor.Executor) (bool, error) {
	changed := false

	// Handle running state
	running, err := s.isRunning(ctx, exec)
	if err != nil {
		return false, err
	}

	if s.state == "running" && !running {
		if _, stderr, exitCode, err := exec.Run(ctx, "systemctl", "start", s.name); err != nil || exitCode != 0 {
			return false, fmt.Errorf("failed to start %s (exit %d): %s", s.name, exitCode, string(stderr))
		}
		changed = true
	} else if s.state == "stopped" && running {
		if _, stderr, exitCode, err := exec.Run(ctx, "systemctl", "stop", s.name); err != nil || exitCode != 0 {
			return false, fmt.Errorf("failed to stop %s (exit %d): %s", s.name, exitCode, string(stderr))
		}
		changed = true
	}

	// Handle enabled state
	enabled, err := s.isEnabled(ctx, exec)
	if err != nil {
		return false, err
	}

	if s.enabled && !enabled {
		if _, stderr, exitCode, err := exec.Run(ctx, "systemctl", "enable", s.name); err != nil || exitCode != 0 {
			return false, fmt.Errorf("failed to enable %s (exit %d): %s", s.name, exitCode, string(stderr))
		}
		changed = true
	} else if !s.enabled && enabled {
		if _, stderr, exitCode, err := exec.Run(ctx, "systemctl", "disable", s.name); err != nil || exitCode != 0 {
			return false, fmt.Errorf("failed to disable %s (exit %d): %s", s.name, exitCode, string(stderr))
		}
		changed = true
	}

	return changed, nil
}

// Restart restarts the service (called via notification)
func (s *ServiceHandler) Restart(ctx context.Context, exec executor.Executor) error {
	_, stderr, exitCode, err := exec.Run(ctx, "systemctl", "restart", s.name)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to restart %s (exit %d): %s", s.name, exitCode, string(stderr))
	}
	return nil
}

// isRunning checks if service is currently active
func (s *ServiceHandler) isRunning(ctx context.Context, exec executor.Executor) (bool, error) {
	stdout, _, _, _ := exec.Run(ctx, "systemctl", "is-active", s.name)
	return strings.TrimSpace(string(stdout)) == "active", nil
}

// isEnabled checks if service is enabled on boot
func (s *ServiceHandler) isEnabled(ctx context.Context, exec executor.Executor) (bool, error) {
	stdout, _, _, _ := exec.Run(ctx, "systemctl", "is-enabled", s.name)
	return strings.TrimSpace(string(stdout)) == "enabled", nil
}
