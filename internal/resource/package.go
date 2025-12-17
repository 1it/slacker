package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// PackageHandler manages package resources via apt-get
type PackageHandler struct {
	name     string
	state    string // "installed" or "absent"
	notifies string
}

// NewPackageHandler creates a PackageHandler from manifest resource
func NewPackageHandler(r manifest.Resource) *PackageHandler {
	state := r.State
	if state == "" {
		state = "installed"
	}
	return &PackageHandler{
		name:     r.Name,
		state:    state,
		notifies: r.Notifies,
	}
}

func (p *PackageHandler) ID() string       { return fmt.Sprintf("package:%s", p.name) }
func (p *PackageHandler) Type() string     { return "package" }
func (p *PackageHandler) Notifies() string { return p.notifies }

// NeedsChange checks if package state differs from desired
func (p *PackageHandler) NeedsChange(ctx context.Context, exec executor.Executor) (bool, error) {
	installed, err := p.isInstalled(ctx, exec)
	if err != nil {
		return false, err
	}

	switch p.state {
	case "installed":
		return !installed, nil
	case "absent":
		return installed, nil
	default:
		return false, fmt.Errorf("unknown package state: %s", p.state)
	}
}

// Apply installs or removes the package
func (p *PackageHandler) Apply(ctx context.Context, exec executor.Executor) (bool, error) {
	needsChange, err := p.NeedsChange(ctx, exec)
	if err != nil {
		return false, err
	}
	if !needsChange {
		return false, nil
	}

	var args []string
	switch p.state {
	case "installed":
		args = []string{"install", "-y", p.name}
	case "absent":
		args = []string{"remove", "-y", p.name}
	}

	// Set non-interactive mode via environment
	_, stderr, exitCode, err := exec.Run(ctx, "apt-get", args...)
	if err != nil || exitCode != 0 {
		return false, fmt.Errorf("apt-get %s %s failed (exit %d): %s", args[0], p.name, exitCode, string(stderr))
	}

	return true, nil
}

// isInstalled checks if package is currently installed
func (p *PackageHandler) isInstalled(ctx context.Context, exec executor.Executor) (bool, error) {
	stdout, _, exitCode, _ := exec.Run(ctx, "dpkg-query", "-W", "-f=${Status}", p.name)
	if exitCode != 0 {
		return false, nil // package not in dpkg database
	}
	status := string(stdout)
	return strings.Contains(status, "install ok installed"), nil
}
