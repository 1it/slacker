package resource

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// UserHandler manages user resources
type UserHandler struct {
	name     string
	uid      int    // -1 means auto-assign
	gid      int    // -1 means auto-assign (use user's primary group)
	home     string // empty means default (/home/<name>)
	shell    string // empty means default (/bin/bash)
	state    string // "present" or "absent"
	system   bool   // create as system user
	notifies string
}

// NewUserHandler creates a UserHandler from manifest resource
func NewUserHandler(r manifest.Resource) *UserHandler {
	state := r.State
	if state == "" {
		state = "present"
	}

	uid := -1
	if r.UID > 0 {
		uid = r.UID
	}

	gid := -1
	if r.GID > 0 {
		gid = r.GID
	}

	return &UserHandler{
		name:     r.Name,
		uid:      uid,
		gid:      gid,
		home:     r.Home,
		shell:    r.Shell,
		state:    state,
		system:   r.System,
		notifies: r.Notifies,
	}
}

func (u *UserHandler) ID() string       { return fmt.Sprintf("user:%s", u.name) }
func (u *UserHandler) Type() string     { return "user" }
func (u *UserHandler) Notifies() string { return u.notifies }

// NeedsChange checks if user state differs from desired
func (u *UserHandler) NeedsChange(ctx context.Context, exec executor.Executor) (bool, error) {
	exists, err := u.userExists(ctx, exec)
	if err != nil {
		return false, err
	}

	switch u.state {
	case "present":
		if !exists {
			return true, nil
		}
		// User exists - check if properties need updating
		return u.propertiesNeedChange(ctx, exec)
	case "absent":
		return exists, nil
	default:
		return false, fmt.Errorf("unknown user state: %s", u.state)
	}
}

// Apply creates, modifies, or removes the user
func (u *UserHandler) Apply(ctx context.Context, exec executor.Executor) (bool, error) {
	needsChange, err := u.NeedsChange(ctx, exec)
	if err != nil {
		return false, err
	}
	if !needsChange {
		return false, nil
	}

	exists, err := u.userExists(ctx, exec)
	if err != nil {
		return false, err
	}

	switch u.state {
	case "present":
		if !exists {
			return true, u.createUser(ctx, exec)
		}
		return true, u.modifyUser(ctx, exec)
	case "absent":
		return true, u.deleteUser(ctx, exec)
	}

	return false, nil
}

// userExists checks if the user exists on the system
func (u *UserHandler) userExists(ctx context.Context, exec executor.Executor) (bool, error) {
	_, _, exitCode, _ := exec.Run(ctx, "id", "-u", u.name)
	return exitCode == 0, nil
}

// propertiesNeedChange checks if existing user properties differ from desired
func (u *UserHandler) propertiesNeedChange(ctx context.Context, exec executor.Executor) (bool, error) {
	// Check UID if specified
	if u.uid > 0 {
		stdout, _, exitCode, _ := exec.Run(ctx, "id", "-u", u.name)
		if exitCode == 0 {
			currentUID, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
			if err == nil && currentUID != u.uid {
				return true, nil
			}
		}
	}

	// Check GID if specified
	if u.gid > 0 {
		stdout, _, exitCode, _ := exec.Run(ctx, "id", "-g", u.name)
		if exitCode == 0 {
			currentGID, err := strconv.Atoi(strings.TrimSpace(string(stdout)))
			if err == nil && currentGID != u.gid {
				return true, nil
			}
		}
	}

	// Check home directory if specified
	if u.home != "" {
		stdout, _, exitCode, _ := exec.Run(ctx, "getent", "passwd", u.name)
		if exitCode == 0 {
			// getent passwd returns: name:x:uid:gid:gecos:home:shell
			parts := strings.Split(string(stdout), ":")
			if len(parts) >= 6 {
				currentHome := strings.TrimSpace(parts[5])
				if currentHome != u.home {
					return true, nil
				}
			}
		}
	}

	// Check shell if specified
	if u.shell != "" {
		stdout, _, exitCode, _ := exec.Run(ctx, "getent", "passwd", u.name)
		if exitCode == 0 {
			parts := strings.Split(string(stdout), ":")
			if len(parts) >= 7 {
				currentShell := strings.TrimSpace(parts[6])
				if currentShell != u.shell {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// createUser creates a new user
func (u *UserHandler) createUser(ctx context.Context, exec executor.Executor) error {
	args := []string{}

	if u.uid > 0 {
		args = append(args, "-u", strconv.Itoa(u.uid))
	}

	if u.gid > 0 {
		args = append(args, "-g", strconv.Itoa(u.gid))
	}

	if u.home != "" {
		args = append(args, "-d", u.home)
	}

	if u.shell != "" {
		args = append(args, "-s", u.shell)
	}

	if u.system {
		args = append(args, "-r") // create system user
	}

	// Create home directory
	args = append(args, "-m")

	// Add username last
	args = append(args, u.name)

	_, stderr, exitCode, err := exec.Run(ctx, "useradd", args...)
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", u.name, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("useradd %s failed (exit %d): %s", u.name, exitCode, string(stderr))
	}

	return nil
}

// modifyUser modifies an existing user
func (u *UserHandler) modifyUser(ctx context.Context, exec executor.Executor) error {
	args := []string{}

	if u.uid > 0 {
		args = append(args, "-u", strconv.Itoa(u.uid))
	}

	if u.gid > 0 {
		args = append(args, "-g", strconv.Itoa(u.gid))
	}

	if u.home != "" {
		args = append(args, "-d", u.home, "-m") // -m to move home contents
	}

	if u.shell != "" {
		args = append(args, "-s", u.shell)
	}

	if len(args) == 0 {
		return nil // nothing to modify
	}

	// Add username last
	args = append(args, u.name)

	_, stderr, exitCode, err := exec.Run(ctx, "usermod", args...)
	if err != nil {
		return fmt.Errorf("failed to modify user %s: %w", u.name, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("usermod %s failed (exit %d): %s", u.name, exitCode, string(stderr))
	}

	return nil
}

// deleteUser removes a user
func (u *UserHandler) deleteUser(ctx context.Context, exec executor.Executor) error {
	// -r removes home directory and mail spool
	_, stderr, exitCode, err := exec.Run(ctx, "userdel", "-r", u.name)
	if err != nil {
		return fmt.Errorf("failed to delete user %s: %w", u.name, err)
	}
	if exitCode != 0 {
		return fmt.Errorf("userdel %s failed (exit %d): %s", u.name, exitCode, string(stderr))
	}

	return nil
}
