package resource

import (
	"context"
	"fmt"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// ExecHandler manages exec resources (arbitrary commands)
type ExecHandler struct {
	name     string
	command  string
	unless   string // skip if this command succeeds
	notifies string
}

// NewExecHandler creates an ExecHandler from manifest resource
func NewExecHandler(r manifest.Resource) *ExecHandler {
	return &ExecHandler{
		name:     r.Name,
		command:  r.Command,
		unless:   r.Unless,
		notifies: r.Notifies,
	}
}

func (e *ExecHandler) ID() string       { return fmt.Sprintf("exec:%s", e.name) }
func (e *ExecHandler) Type() string     { return "exec" }
func (e *ExecHandler) Notifies() string { return e.notifies }

// NeedsChange checks if the "unless" condition is not met
func (e *ExecHandler) NeedsChange(ctx context.Context, exec executor.Executor) (bool, error) {
	if e.unless == "" {
		// No condition, always needs to run
		return true, nil
	}

	// Run the unless command via shell
	_, _, exitCode, _ := exec.Run(ctx, "sh", "-c", e.unless)
	// If unless command succeeds (exit 0), we don't need change
	return exitCode != 0, nil
}

// Apply executes the command
func (e *ExecHandler) Apply(ctx context.Context, exec executor.Executor) (bool, error) {
	needsChange, err := e.NeedsChange(ctx, exec)
	if err != nil {
		return false, err
	}
	if !needsChange {
		return false, nil
	}

	// Run command via shell
	_, stderr, exitCode, err := exec.Run(ctx, "sh", "-c", e.command)
	if err != nil || exitCode != 0 {
		return false, fmt.Errorf("exec %s failed (exit %d): %s", e.name, exitCode, string(stderr))
	}

	return true, nil
}
