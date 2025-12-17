package executor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// LocalExecutor executes commands on the local machine
type LocalExecutor struct{}

// Run executes a command locally and returns stdout, stderr, exit code
func (e *LocalExecutor) Run(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
	c := exec.CommandContext(ctx, cmd, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	c.Stdout = &stdoutBuf
	c.Stderr = &stderrBuf
	// Set DEBIAN_FRONTEND to noninteractive to avoid prompts
	c.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	err = c.Run()

	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()
	exitCode = c.ProcessState.ExitCode()

	// Don't treat non-zero exit as error - let caller decide
	if err != nil && exitCode != 0 {
		err = nil
	}

	return stdout, stderr, exitCode, err
}

// WriteFile writes content to a file with specified mode
func (e *LocalExecutor) WriteFile(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	return os.WriteFile(path, content, mode)
}

// ReadFile reads the content of a file
func (e *LocalExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Stat returns file info for the given path
func (e *LocalExecutor) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// Chown changes ownership of a file
func (e *LocalExecutor) Chown(ctx context.Context, path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
