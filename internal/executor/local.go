package executor

import (
	"context"
	"os"
	"os/exec"
)

type LocalExecutor struct {
	runner *exec.Cmd
}

func (e *LocalExecutor) Run(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
	e.runner = exec.CommandContext(ctx, cmd, args...)
	output, err := e.runner.Output()
	return output, nil, 0, err
}

func (e *LocalExecutor) WriteFile(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	return os.WriteFile(path, content, mode)
}

func (e *LocalExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (e *LocalExecutor) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (e *LocalExecutor) Chown(ctx context.Context, path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
