package resource

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"

	"github.com/1it/slacker/internal/executor"
	"github.com/1it/slacker/internal/manifest"
)

// FileHandler manages file resources
type FileHandler struct {
	name     string
	path     string
	content  string
	owner    string
	group    string
	mode     string
	notifies string
}

// NewFileHandler creates a FileHandler from manifest resource
func NewFileHandler(r manifest.Resource) *FileHandler {
	name := r.Name
	if name == "" {
		name = r.Path
	}
	return &FileHandler{
		name:     name,
		path:     r.Path,
		content:  r.Content,
		owner:    r.Owner,
		group:    r.Group,
		mode:     r.Mode,
		notifies: r.Notifies,
	}
}

func (f *FileHandler) ID() string       { return fmt.Sprintf("file:%s", f.path) }
func (f *FileHandler) Type() string     { return "file" }
func (f *FileHandler) Notifies() string { return f.notifies }

// NeedsChange checks if file content or permissions differ from desired state
func (f *FileHandler) NeedsChange(ctx context.Context, exec executor.Executor) (bool, error) {
	existing, err := exec.ReadFile(ctx, f.path)
	if err != nil {
		// File doesn't exist - needs change
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to read file %s: %w", f.path, err)
	}

	// Compare content hash
	existingHash := sha256.Sum256(existing)
	desiredHash := sha256.Sum256([]byte(f.content))
	if existingHash != desiredHash {
		return true, nil
	}

	// Compare mode if specified
	if f.mode != "" {
		info, err := exec.Stat(ctx, f.path)
		if err != nil {
			return false, fmt.Errorf("failed to stat file %s: %w", f.path, err)
		}
		desiredMode, err := strconv.ParseUint(f.mode, 8, 32)
		if err != nil {
			return false, fmt.Errorf("invalid mode %s: %w", f.mode, err)
		}
		if info.Mode().Perm() != os.FileMode(desiredMode) {
			return true, nil
		}
	}

	return false, nil
}

// Apply writes the file with specified content and permissions
func (f *FileHandler) Apply(ctx context.Context, exec executor.Executor) (bool, error) {
	needsChange, err := f.NeedsChange(ctx, exec)
	if err != nil {
		return false, err
	}
	if !needsChange {
		return false, nil
	}

	// Determine file mode
	mode := os.FileMode(0644)
	if f.mode != "" {
		parsed, err := strconv.ParseUint(f.mode, 8, 32)
		if err != nil {
			return false, fmt.Errorf("invalid mode %s: %w", f.mode, err)
		}
		mode = os.FileMode(parsed)
	}

	// Write file
	if err := exec.WriteFile(ctx, f.path, []byte(f.content), mode); err != nil {
		return false, fmt.Errorf("failed to write file %s: %w", f.path, err)
	}

	// Set ownership if specified
	if f.owner != "" || f.group != "" {
		uid, gid, err := f.resolveOwnership(ctx, exec)
		if err != nil {
			return false, err
		}
		if err := exec.Chown(ctx, f.path, uid, gid); err != nil {
			return false, fmt.Errorf("failed to chown file %s: %w", f.path, err)
		}
	}

	return true, nil
}

// resolveOwnership converts owner/group names to UID/GID
func (f *FileHandler) resolveOwnership(ctx context.Context, exec executor.Executor) (uid, gid int, err error) {
	uid, gid = -1, -1

	if f.owner != "" {
		stdout, stderr, exitCode, err := exec.Run(ctx, "id", "-u", f.owner)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to resolve user %s: %w", f.owner, err)
		}
		if exitCode != 0 {
			return 0, 0, fmt.Errorf("failed to resolve user %s: command exited with code %d: %s", f.owner, exitCode, string(stderr))
		}
		if len(stdout) == 0 {
			return 0, 0, fmt.Errorf("failed to resolve user %s: empty output from id command", f.owner)
		}
		parsed, err := strconv.Atoi(string(stdout[:len(stdout)-1])) // trim newline
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse uid for %s: %w", f.owner, err)
		}
		uid = parsed
	}

	if f.group != "" {
		stdout, _, exitCode, err := exec.Run(ctx, "getent", "group", f.group)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to resolve group %s: %w", f.group, err)
		}
		// If getent fails (non-zero exit or empty output), fallback to id -g
		if exitCode != 0 || len(stdout) == 0 {
			stdout, stderr, exitCode, err := exec.Run(ctx, "id", "-g", f.group)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to resolve group %s: %w", f.group, err)
			}
			if exitCode != 0 {
				return 0, 0, fmt.Errorf("failed to resolve group %s: command exited with code %d: %s", f.group, exitCode, string(stderr))
			}
			if len(stdout) == 0 {
				return 0, 0, fmt.Errorf("failed to resolve group %s: empty output from id command", f.group)
			}
			parsed, err := strconv.Atoi(string(stdout[:len(stdout)-1]))
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse gid for %s: %w", f.group, err)
			}
			gid = parsed
		} else {
			// getent group returns: name:password:gid:members
			var gidStr string
			if _, err := fmt.Sscanf(string(stdout), "%*[^:]:%*[^:]:%s", &gidStr); err != nil {
				return 0, 0, fmt.Errorf("failed to parse getent output for group %s: %w", f.group, err)
			}
			parsed, err := strconv.Atoi(gidStr)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse gid for %s from getent output: %w", f.group, err)
			}
			gid = parsed
		}
	}

	return uid, gid, nil
}
