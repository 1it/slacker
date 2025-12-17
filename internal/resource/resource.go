package resource

import (
	"context"

	"github.com/1it/slacker/internal/executor"
)

// Resource is a generic resource that can be applied to a system.
type Resource interface {
	ID() string
	NeedsChange(ctx context.Context, exec executor.Executor) (bool, error)
	Apply(ctx context.Context, exec executor.Executor) (changed bool, err error)
}
