package executor

import (
	"context"
	"os"
)

// MockExecutor is a test double for the Executor interface
type MockExecutor struct {
	RunFunc       func(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error)
	WriteFileFunc func(ctx context.Context, path string, content []byte, mode os.FileMode) error
	ReadFileFunc  func(ctx context.Context, path string) ([]byte, error)
	StatFunc      func(ctx context.Context, path string) (os.FileInfo, error)
	ChownFunc     func(ctx context.Context, path string, uid, gid int) error
}

func (m *MockExecutor) Run(ctx context.Context, cmd string, args ...string) (stdout, stderr []byte, exitCode int, err error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, cmd, args...)
	}
	return nil, nil, 0, nil
}

func (m *MockExecutor) WriteFile(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, path, content, mode)
	}
	return nil
}

func (m *MockExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(ctx, path)
	}
	return nil, nil
}

func (m *MockExecutor) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(ctx, path)
	}
	return nil, nil
}

func (m *MockExecutor) Chown(ctx context.Context, path string, uid, gid int) error {
	if m.ChownFunc != nil {
		return m.ChownFunc(ctx, path, uid, gid)
	}
	return nil
}
