package executor

import (
	"context"
	"os"
)

type Executor interface {
	Run(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error)
	WriteFile(ctx context.Context, path string, content []byte, mode os.FileMode) error
	ReadFile(ctx context.Context, path string) ([]byte, error)
	Stat(ctx context.Context, path string) (os.FileInfo, error)
	Chown(ctx context.Context, path string, uid, gid int) error
}
